package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
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
)

type KieSongGenerator struct {
	apiKey string
	client *http.Client
}

func NewKieSongGenerator(apiKey string, _ StorageService, _ string) *KieSongGenerator {
	return &KieSongGenerator{
		apiKey: apiKey,
		client: &http.Client{Timeout: 60 * time.Second},
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
		"title":        "FunBaza",
		"model":        kieMusicModel,
		"callBackUrl":  callbackURL,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal generate request: %w", err)
	}
	return g.submitMusicTask(ctx, "/api/v1/generate", body)
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
