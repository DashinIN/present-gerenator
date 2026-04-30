package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	kieMusicBaseURL      = "https://api.kie.ai"
	kieMusicModel        = "V4"
	kieMusicPollInterval = 5 * time.Second
	kieMusicTimeout      = 10 * time.Minute
	kieLyricsTimeout     = 3 * time.Minute
	kieNoopCallbackURL   = "https://example.com/noop"
	kieMashupTempoFactor = 1.05
	kieMashupAudioWeight = 0.65
	kieMashupStyleWeight = 0.35
	kieMashupWeirdness   = 0.25
)

type KieSongGenerator struct {
	apiKey     string
	storage    StorageService
	client     *http.Client
	ffmpegPath string
}

func NewKieSongGenerator(apiKey string, storage StorageService, ffmpegPath string) *KieSongGenerator {
	if strings.TrimSpace(ffmpegPath) == "" {
		ffmpegPath = "ffmpeg"
	}
	return &KieSongGenerator{
		apiKey:     apiKey,
		storage:    storage,
		client:     &http.Client{Timeout: 60 * time.Second},
		ffmpegPath: ffmpegPath,
	}
}

func (g *KieSongGenerator) Generate(ctx context.Context, lyrics, style string, count int) ([][]byte, error) {
	taskID, err := g.submitGenerate(ctx, lyrics, style, kieNoopCallbackURL)
	if err != nil {
		return nil, fmt.Errorf("kie generate submit: %w", err)
	}

	clips, err := g.pollMusicTask(ctx, taskID, nil)
	if err != nil {
		return nil, fmt.Errorf("kie generate poll: %w", err)
	}
	return g.downloadClips(ctx, clips, count)
}

func (g *KieSongGenerator) GenerateMashup(ctx context.Context, req MashupSongRequest) ([][]byte, error) {
	return g.GenerateMashupStreaming(ctx, req, nil)
}

func (g *KieSongGenerator) GenerateMashupStreaming(ctx context.Context, req MashupSongRequest, onPartial func([][]byte)) ([][]byte, error) {
	if len(req.InputAudioKeys) == 0 {
		return nil, errors.New("mashup requires at least one input track")
	}

	urls, err := g.ensureMashupURLs(ctx, req.InputAudioKeys)
	if err != nil {
		return nil, err
	}

	clips, err := g.generateMashupWithURLs(ctx, urls, req, onPartial)
	if err != nil {
		if isCopyrightError(err) {
			return nil, fmt.Errorf("copyright violation: processed uploaded audio could not pass provider checks")
		}
		return nil, err
	}
	return g.downloadClips(ctx, clips, 1)
}

func (g *KieSongGenerator) GenerateLyrics(ctx context.Context, prompt string) (string, string, error) {
	body, err := json.Marshal(map[string]string{
		"prompt":      prompt,
		"callBackUrl": kieNoopCallbackURL,
	})
	if err != nil {
		return "", "", fmt.Errorf("marshal lyrics request: %w", err)
	}

	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			TaskID string `json:"taskId"`
		} `json:"data"`
	}
	if err := g.postJSON(ctx, "/api/v1/lyrics", body, &resp); err != nil {
		return "", "", err
	}
	if resp.Code != 200 {
		return "", "", fmt.Errorf("lyrics submit error %d: %s", resp.Code, resp.Msg)
	}

	deadline := time.Now().Add(kieLyricsTimeout)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case <-ticker.C:
		}
		if time.Now().After(deadline) {
			return "", "", errors.New("lyrics generation timeout")
		}

		var info struct {
			Code int `json:"code"`
			Data struct {
				Response struct {
					Data []struct {
						Text         string `json:"text"`
						Title        string `json:"title"`
						Status       string `json:"status"`
						ErrorMessage string `json:"error_message"`
					} `json:"data"`
				} `json:"response"`
			} `json:"data"`
		}
		if err := g.getJSON(ctx, "/api/v1/lyrics/record-info?taskId="+resp.Data.TaskID, &info); err != nil {
			slog.Warn("kie lyrics poll error", "err", err)
			continue
		}
		if info.Code != 200 || len(info.Data.Response.Data) == 0 {
			continue
		}

		item := info.Data.Response.Data[0]
		switch strings.ToLower(item.Status) {
		case "complete", "success":
			return item.Text, item.Title, nil
		case "error", "failed":
			return "", "", fmt.Errorf("lyrics task failed: %s", item.ErrorMessage)
		}
	}
}

type kieMusicClip struct {
	ID       string  `json:"id"`
	AudioURL string  `json:"audioUrl"`
	Title    string  `json:"title"`
	Duration float64 `json:"duration"`
}

func (g *KieSongGenerator) submitGenerate(ctx context.Context, lyrics, style, callbackURL string) (string, error) {
	payload := map[string]any{
		"customMode":   true,
		"instrumental": false,
		"prompt":       lyrics,
		"style":        style,
		"title":        "FunGreet",
		"model":        kieMusicModel,
		"callBackUrl":  callbackURL,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal generate request: %w", err)
	}
	return g.submitMusicTask(ctx, "/api/v1/generate", body)
}

func (g *KieSongGenerator) generateMashupWithURLs(ctx context.Context, uploadURLs []string, req MashupSongRequest, onPartial func([][]byte)) ([]kieMusicClip, error) {
	payload := map[string]any{
		"uploadUrlList": ensureTwoURLs(uploadURLs),
		"model":         kieMusicModel,
		"callBackUrl":   kieNoopCallbackURL,
	}

	lyrics := strings.TrimSpace(req.Lyrics)
	style := strings.TrimSpace(req.Style)
	prompt := strings.TrimSpace(req.Prompt)
	if lyrics != "" || style != "" {
		payload["customMode"] = true
		payload["instrumental"] = false
		if lyrics != "" {
			payload["prompt"] = lyrics
		} else if prompt != "" {
			payload["prompt"] = prompt
		}
		if style != "" {
			payload["style"] = style
		}
		payload["audioWeight"] = kieMashupAudioWeight
		payload["styleWeight"] = kieMashupStyleWeight
		payload["weirdnessConstraint"] = kieMashupWeirdness
		payload["title"] = "FunGreet Mashup"
	} else {
		payload["customMode"] = false
		payload["instrumental"] = true
		if prompt == "" {
			prompt = "Mashup of uploaded audio tracks"
		}
		payload["prompt"] = prompt
	}

	slog.Info("kie mashup submit",
		"tracks", len(uploadURLs),
		"customMode", payload["customMode"],
		"style", style,
		"hasLyrics", lyrics != "",
		"instrumental", payload["instrumental"],
		"audioWeight", payload["audioWeight"],
		"styleWeight", payload["styleWeight"],
	)

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal mashup request: %w", err)
	}

	taskID, err := g.submitMusicTask(ctx, "/api/v1/generate/mashup", body)
	if err != nil {
		return nil, err
	}
	return g.pollMusicTask(ctx, taskID, func(clips []kieMusicClip) {
		if onPartial == nil || len(clips) == 0 {
			return
		}
		data, err := g.downloadClips(ctx, clips, 0)
		if err != nil {
			slog.Warn("kie mashup partial download failed", "task_id", taskID, "err", err)
			return
		}
		onPartial(data)
	})
}

func (g *KieSongGenerator) submitMusicTask(ctx context.Context, path string, body []byte) (string, error) {
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			TaskID string `json:"taskId"`
		} `json:"data"`
	}
	if err := g.postJSON(ctx, path, body, &resp); err != nil {
		return "", err
	}
	if resp.Code != 200 {
		return "", fmt.Errorf("music submit error %d: %s", resp.Code, resp.Msg)
	}
	return resp.Data.TaskID, nil
}

func (g *KieSongGenerator) pollMusicTask(ctx context.Context, taskID string, onPartial func([]kieMusicClip)) ([]kieMusicClip, error) {
	deadline := time.Now().Add(kieMusicTimeout)
	ticker := time.NewTicker(kieMusicPollInterval)
	defer ticker.Stop()
	partialFired := false

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("music generation timeout after %v", kieMusicTimeout)
		}

		var info struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
			Data struct {
				Status       string `json:"status"`
				ErrorCode    any    `json:"errorCode"`
				ErrorMessage string `json:"errorMessage"`
				Response     struct {
					SunoData []kieMusicClip `json:"sunoData"`
				} `json:"response"`
			} `json:"data"`
		}
		if err := g.getJSON(ctx, "/api/v1/generate/record-info?taskId="+taskID, &info); err != nil {
			slog.Warn("kie music poll error", "task_id", taskID, "err", err)
			continue
		}
		if info.Code != 200 {
			slog.Warn("kie music poll non-200", "task_id", taskID, "code", info.Code, "msg", info.Msg)
			continue
		}

		status := info.Data.Status
		ready := make([]kieMusicClip, 0, len(info.Data.Response.SunoData))
		for _, clip := range info.Data.Response.SunoData {
			if clip.AudioURL != "" {
				ready = append(ready, clip)
			}
		}

		switch status {
		case "SUCCESS":
			if len(ready) == 0 {
				return nil, errors.New("task SUCCESS but no audio URLs")
			}
			return ready, nil
		case "FIRST_SUCCESS":
			if !partialFired && onPartial != nil && len(ready) > 0 {
				partialFired = true
				onPartial(ready)
			}
			continue
		case "PENDING", "TEXT_SUCCESS":
			continue
		case "CREATE_TASK_FAILED", "GENERATE_AUDIO_FAILED", "CALLBACK_EXCEPTION", "SENSITIVE_WORD_ERROR":
			return nil, fmt.Errorf("kie task failed: status=%s error_code=%v error_message=%s", status, info.Data.ErrorCode, info.Data.ErrorMessage)
		default:
			slog.Info("kie music poll waiting", "task_id", taskID, "status", status)
		}
	}
}

func (g *KieSongGenerator) downloadClips(ctx context.Context, clips []kieMusicClip, count int) ([][]byte, error) {
	if count > 0 && len(clips) > count {
		clips = clips[:count]
	}

	result := make([][]byte, len(clips))
	for i, clip := range clips {
		data, err := g.download(ctx, clip.AudioURL)
		if err != nil {
			return nil, fmt.Errorf("download clip %d: %w", i, err)
		}
		result[i] = data
	}
	return result, nil
}

func (g *KieSongGenerator) ensureMashupURLs(ctx context.Context, keys []string) ([]string, error) {
	urls := make([]string, 0, len(keys))
	for _, key := range keys {
		data, err := g.storage.Download(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("download input audio %s: %w", key, err)
		}
		filename := filepath.Base(key)
		data, filename, err = g.preprocessMashupAudio(ctx, filename, data)
		if err != nil {
			return nil, fmt.Errorf("preprocess input audio %s: %w", key, err)
		}
		url, err := g.uploadToKie(ctx, filename, data)
		if err != nil {
			return nil, fmt.Errorf("upload input audio %s to kie: %w", key, err)
		}
		urls = append(urls, url)
	}
	return ensureTwoURLs(urls), nil
}

func (g *KieSongGenerator) preprocessMashupAudio(ctx context.Context, filename string, data []byte) ([]byte, string, error) {
	if strings.ToLower(filepath.Ext(filename)) != ".mp3" {
		return data, filename, nil
	}

	processed, err := g.runFFmpegMashupPreprocess(ctx, data)
	if err != nil {
		return nil, "", err
	}
	slog.Info("kie mashup audio preprocessed", "file", filename, "tempo", kieMashupTempoFactor, "max_seconds", 60)
	return processed, filename, nil
}

func (g *KieSongGenerator) runFFmpegMashupPreprocess(ctx context.Context, data []byte) ([]byte, error) {
	tmpDir, err := os.MkdirTemp("", "fungreet-mashup-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	inputPath := filepath.Join(tmpDir, "input.mp3")
	outputPath := filepath.Join(tmpDir, "output.mp3")
	if err := os.WriteFile(inputPath, data, 0o600); err != nil {
		return nil, fmt.Errorf("write temp input: %w", err)
	}

	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-y",
		"-i", inputPath,
		"-vn",
		"-filter:a", fmt.Sprintf("atempo=%.2f,atrim=end=60", kieMashupTempoFactor),
		"-c:a", "libmp3lame",
		"-q:a", "2",
		outputPath,
	}
	cmd := exec.CommandContext(ctx, g.ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf("ffmpeg not found: set FFMPEG_PATH or add ffmpeg to PATH")
		}
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("ffmpeg failed: %s", msg)
		}
		return nil, fmt.Errorf("ffmpeg failed: %w", err)
	}

	out, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("read temp output: %w", err)
	}
	if len(out) == 0 {
		return nil, errors.New("ffmpeg produced empty output")
	}
	return out, nil
}

func ensureTwoURLs(urls []string) []string {
	if len(urls) == 1 {
		return []string{urls[0], urls[0]}
	}
	return urls
}

func (g *KieSongGenerator) uploadToKie(ctx context.Context, filename string, data []byte) (string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("uploadPath", "fungreet/audio")
	_ = mw.WriteField("fileName", filename)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := fw.Write(data); err != nil {
		return "", err
	}
	if err := mw.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, kieUploadURL, &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+g.apiKey)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := g.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("kie upload: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var result struct {
		Success bool   `json:"success"`
		Code    int    `json:"code"`
		Msg     string `json:"msg"`
		Data    struct {
			DownloadURL string `json:"downloadUrl"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("kie upload decode: %w", err)
	}
	if !result.Success && result.Code != 200 {
		return "", fmt.Errorf("kie upload error %d: %s", result.Code, result.Msg)
	}
	if result.Data.DownloadURL == "" {
		return "", fmt.Errorf("kie upload returned empty downloadUrl")
	}
	return result.Data.DownloadURL, nil
}

func (g *KieSongGenerator) postJSON(ctx context.Context, path string, body []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, kieMusicBaseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+g.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return json.Unmarshal(raw, out)
}

func (g *KieSongGenerator) getJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, kieMusicBaseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+g.apiKey)

	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return json.Unmarshal(raw, out)
}

func (g *KieSongGenerator) download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download audio http %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func isCopyrightError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "copyright") ||
		strings.Contains(msg, "work of art") ||
		strings.Contains(msg, "matches existing") ||
		strings.Contains(msg, "error_code=413") ||
		strings.Contains(msg, "code 413")
}
