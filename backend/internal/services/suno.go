package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	sunoAPIBase      = "https://api.sunoapi.org"
	sunoModel        = "V4"
	sunoMaxPollTime  = 10 * time.Minute
	sunoPollInterval = 5 * time.Second
	// callBackUrl обязателен по схеме, но мы используем polling — указываем заглушку
	sunoCallbackURL = "https://example.com/noop"
)

type SunoAPIGenerator struct {
	apiKey string
	client *http.Client
}

func NewSunoAPIGenerator(apiKey string) *SunoAPIGenerator {
	return &SunoAPIGenerator{
		apiKey: apiKey,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Generate реализует SongGenerator: lyrics = текст песни, style = теги стиля.
func (g *SunoAPIGenerator) Generate(ctx context.Context, lyrics, style string, count int) ([][]byte, error) {
	taskID, err := g.submitGenerate(ctx, lyrics, style)
	if err != nil {
		return nil, fmt.Errorf("suno submit: %w", err)
	}
	slog.Info("suno task submitted", "task_id", taskID)

	clips, err := g.pollTask(ctx, taskID, nil)
	if err != nil {
		return nil, fmt.Errorf("suno poll: %w", err)
	}

	result := make([][]byte, len(clips))
	for i, clip := range clips {
		data, err := g.download(ctx, clip.AudioURL)
		if err != nil {
			return nil, fmt.Errorf("suno download clip %d: %w", i, err)
		}
		result[i] = data
	}
	return result, nil
}

// GenerateStreaming реализует StreamingSongGenerator: вызывает onPartial как только первый клип готов.
// Работает и при FIRST_SUCCESS, и когда Suno пропускает его и сразу отдаёт SUCCESS.
func (g *SunoAPIGenerator) GenerateStreaming(ctx context.Context, lyrics, style string, count int, onPartial func([][]byte)) ([][]byte, error) {
	taskID, err := g.submitGenerate(ctx, lyrics, style)
	if err != nil {
		return nil, fmt.Errorf("suno submit: %w", err)
	}
	slog.Info("suno task submitted (streaming)", "task_id", taskID)

	downloaded := map[string][]byte{}
	partialFired := false

	clips, err := g.pollTask(ctx, taskID, func(partial []sunoClip) {
		// FIRST_SUCCESS — скачиваем и сохраняем сразу
		if onPartial == nil {
			return
		}
		data := make([][]byte, 0, len(partial))
		for _, clip := range partial {
			b, err := g.download(ctx, clip.AudioURL)
			if err != nil {
				slog.Warn("suno partial download failed", "err", err)
				continue
			}
			downloaded[clip.AudioURL] = b
			data = append(data, b)
		}
		if len(data) > 0 {
			partialFired = true
			onPartial(data)
		}
	})
	if err != nil {
		return nil, fmt.Errorf("suno poll: %w", err)
	}

	// Скачиваем финальные клипы, пропуская уже скачанные.
	// Если FIRST_SUCCESS не было (Suno прыгнул сразу в SUCCESS),
	// сохраняем первый клип через onPartial как только он скачается.
	slog.Info("suno streaming: final download", "task_id", taskID, "clips", len(clips), "partialFired", partialFired)
	result := make([][]byte, len(clips))
	for i, clip := range clips {
		if b, ok := downloaded[clip.AudioURL]; ok {
			slog.Info("suno streaming: clip from cache", "task_id", taskID, "i", i)
			result[i] = b
			continue
		}
		slog.Info("suno streaming: downloading clip", "task_id", taskID, "i", i)
		b, err := g.download(ctx, clip.AudioURL)
		if err != nil {
			return nil, fmt.Errorf("suno download clip %d: %w", i, err)
		}
		result[i] = b
		downloaded[clip.AudioURL] = b
		slog.Info("suno streaming: clip downloaded", "task_id", taskID, "i", i, "partialFired", partialFired)

		// Если partial ещё не был отправлен — сохраняем первый скачанный клип сразу
		if !partialFired && onPartial != nil {
			partialFired = true
			slog.Info("suno streaming: firing partial from SUCCESS loop", "task_id", taskID, "i", i)
			onPartial([][]byte{b})
		}
	}
	return result, nil
}

// GenerateExtend генерирует продолжение трека по его audioId из sunoapi.org.
func (g *SunoAPIGenerator) GenerateExtend(ctx context.Context, audioID, lyrics, style string, count int) ([][]byte, error) {
	taskID, err := g.submitExtend(ctx, audioID, lyrics, style)
	if err != nil {
		return nil, fmt.Errorf("suno extend submit: %w", err)
	}
	slog.Info("suno extend task submitted", "task_id", taskID)

	clips, err := g.pollTask(ctx, taskID, nil)
	if err != nil {
		return nil, fmt.Errorf("suno extend poll: %w", err)
	}

	if len(clips) > count {
		clips = clips[:count]
	}

	result := make([][]byte, len(clips))
	for i, clip := range clips {
		data, err := g.download(ctx, clip.AudioURL)
		if err != nil {
			return nil, fmt.Errorf("suno extend download clip %d: %w", i, err)
		}
		result[i] = data
	}
	return result, nil
}

// GenerateLyrics генерирует текст песни по описанию через /api/v1/lyrics.
func (g *SunoAPIGenerator) GenerateLyrics(ctx context.Context, prompt string) (string, string, error) {
	body, _ := json.Marshal(map[string]string{
		"prompt":      prompt,
		"callBackUrl": sunoCallbackURL,
	})

	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			TaskID string `json:"taskId"`
		} `json:"data"`
	}
	if err := g.post(ctx, "/api/v1/lyrics", body, &resp); err != nil {
		return "", "", err
	}
	if resp.Code != 200 {
		return "", "", fmt.Errorf("lyrics submit error %d: %s", resp.Code, resp.Msg)
	}

	taskID := resp.Data.TaskID
	slog.Info("suno lyrics task submitted", "task_id", taskID)

	deadline := time.Now().Add(3 * time.Minute)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case <-ticker.C:
		}
		if time.Now().After(deadline) {
			return "", "", fmt.Errorf("lyrics generation timeout")
		}

		var info struct {
			Code int `json:"code"`
			Data struct {
				Response struct {
					Data []struct {
						Text         string `json:"text"`
						Title        string `json:"title"`
						Status       string `json:"status"`
						ErrorMessage string `json:"errorMessage"`
					} `json:"data"`
				} `json:"response"`
			} `json:"data"`
		}
		rawBytes, pollErr := g.getRaw(ctx, "/api/v1/lyrics/record-info?taskId="+taskID)
		if pollErr != nil {
			slog.Warn("suno lyrics poll error", "err", pollErr)
			continue
		}
		slog.Info("suno lyrics poll raw", "task_id", taskID, "body", string(rawBytes))
		if err := json.Unmarshal(rawBytes, &info); err != nil {
			slog.Warn("suno lyrics poll unmarshal error", "err", err)
			continue
		}
		if info.Code != 200 {
			slog.Warn("suno lyrics poll non-200", "code", info.Code)
			continue
		}
		if len(info.Data.Response.Data) > 0 {
			st := info.Data.Response.Data[0].Status
			slog.Info("suno lyrics status", "task_id", taskID, "status", st)
			switch strings.ToLower(st) {
			case "complete", "success":
				return info.Data.Response.Data[0].Text, info.Data.Response.Data[0].Title, nil
			case "error", "failed":
				return "", "", fmt.Errorf("lyrics task failed: %s", info.Data.Response.Data[0].ErrorMessage)
			}
		}
	}
}

// --- internal ---

type sunoGenerateRequest struct {
	CustomMode  bool   `json:"customMode"`
	Instrumental bool  `json:"instrumental"`
	Prompt      string `json:"prompt"`
	Style       string `json:"style"`
	Title       string `json:"title"`
	Model       string `json:"model"`
	CallBackURL string `json:"callBackUrl"`
}

type sunoExtendRequest struct {
	DefaultParamFlag bool   `json:"defaultParamFlag"`
	AudioID          string `json:"audioId"`
	Prompt           string `json:"prompt,omitempty"`
	Style            string `json:"style,omitempty"`
	Title            string `json:"title,omitempty"`
	Model            string `json:"model"`
	CallBackURL      string `json:"callBackUrl"`
}

type sunoClip struct {
	ID       string `json:"id"`
	AudioURL string `json:"audioUrl"`
	Title    string `json:"title"`
	Duration float64 `json:"duration"`
}

func (g *SunoAPIGenerator) submitGenerate(ctx context.Context, lyrics, style string) (string, error) {
	body, _ := json.Marshal(sunoGenerateRequest{
		CustomMode:   true,
		Instrumental: false,
		Prompt:       lyrics,
		Style:        style,
		Title:        "FunGreet",
		Model:        sunoModel,
		CallBackURL:  sunoCallbackURL,
	})

	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			TaskID string `json:"taskId"`
		} `json:"data"`
	}
	if err := g.post(ctx, "/api/v1/generate", body, &resp); err != nil {
		return "", err
	}
	if resp.Code != 200 {
		return "", fmt.Errorf("generate error %d: %s", resp.Code, resp.Msg)
	}
	return resp.Data.TaskID, nil
}

func (g *SunoAPIGenerator) submitExtend(ctx context.Context, audioID, lyrics, style string) (string, error) {
	req := sunoExtendRequest{
		DefaultParamFlag: lyrics != "",
		AudioID:          audioID,
		Model:            sunoModel,
		CallBackURL:      sunoCallbackURL,
	}
	if lyrics != "" {
		req.Prompt = lyrics
		req.Style = style
		req.Title = "FunGreet"
	}

	body, _ := json.Marshal(req)

	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			TaskID string `json:"taskId"`
		} `json:"data"`
	}
	if err := g.post(ctx, "/api/v1/generate/extend", body, &resp); err != nil {
		return "", err
	}
	if resp.Code != 200 {
		return "", fmt.Errorf("extend error %d: %s", resp.Code, resp.Msg)
	}
	return resp.Data.TaskID, nil
}

func (g *SunoAPIGenerator) pollTask(ctx context.Context, taskID string, onPartial func([]sunoClip)) ([]sunoClip, error) {
	deadline := time.Now().Add(sunoMaxPollTime)
	ticker := time.NewTicker(sunoPollInterval)
	defer ticker.Stop()

	partialFired := false

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("generation timeout after %v", sunoMaxPollTime)
		}

		var info struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
			Data struct {
				Status   string `json:"status"`
				Response struct {
					SunoData []sunoClip `json:"sunoData"`
				} `json:"response"`
			} `json:"data"`
		}
		if err := g.get(ctx, "/api/v1/generate/record-info?taskId="+taskID, &info); err != nil {
			slog.Warn("suno poll error, retrying", "err", err)
			continue
		}
		if info.Code != 200 {
			slog.Warn("suno poll non-200", "code", info.Code, "msg", info.Msg)
			continue
		}

		status := info.Data.Status
		slog.Info("suno poll", "task_id", taskID, "status", status)

		ready := make([]sunoClip, 0, len(info.Data.Response.SunoData))
		for _, c := range info.Data.Response.SunoData {
			if c.AudioURL != "" {
				ready = append(ready, c)
			}
		}

		switch status {
		case "SUCCESS":
			if len(ready) == 0 {
				return nil, fmt.Errorf("task SUCCESS but no audio URLs")
			}
			return ready, nil
		case "FIRST_SUCCESS":
			if !partialFired && onPartial != nil && len(ready) > 0 {
				partialFired = true
				onPartial(ready)
			}
		case "CREATE_TASK_FAILED", "GENERATE_AUDIO_FAILED", "SENSITIVE_WORD_ERROR":
			return nil, fmt.Errorf("suno task failed: %s", status)
		}
	}
}

func (g *SunoAPIGenerator) download(ctx context.Context, audioURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, audioURL, nil)
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


func (g *SunoAPIGenerator) post(ctx context.Context, path string, body []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		sunoAPIBase+path, bytes.NewReader(body))
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
		return fmt.Errorf("http %d: %s", resp.StatusCode, raw)
	}
	return json.Unmarshal(raw, out)
}

func (g *SunoAPIGenerator) getRaw(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sunoAPIBase+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+g.apiKey)
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return raw, nil
}

func (g *SunoAPIGenerator) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		sunoAPIBase+path, nil)
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
