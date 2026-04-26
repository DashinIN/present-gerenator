package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"time"
)

const (
	kieBaseURL         = "https://api.kie.ai/api/v1"
	kieUploadURL       = "https://api.kie.ai/api/file-stream-upload"
	kiePollingInterval = 5 * time.Second
	kieTimeout         = 5 * time.Minute
)

type KieImageGenerator struct {
	apiKey  string
	storage StorageService
	client  *http.Client
}

func NewKieImageGenerator(apiKey string, storage StorageService) *KieImageGenerator {
	return &KieImageGenerator{
		apiKey:  apiKey,
		storage: storage,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

// Submit реализует AsyncImageGenerator.
func (g *KieImageGenerator) Submit(ctx context.Context, prompt string, refImages []string, callbackURL string) (string, error) {
	publicURLs, err := g.ensurePublicURLs(ctx, refImages)
	if err != nil {
		return "", err
	}
	return g.submit(ctx, prompt, publicURLs, callbackURL)
}

func (g *KieImageGenerator) Generate(ctx context.Context, prompt string, refImages []string, count int) ([][]byte, error) {
	publicURLs, err := g.ensurePublicURLs(ctx, refImages)
	if err != nil {
		return nil, err
	}

	taskID, err := g.submit(ctx, prompt, publicURLs, "")
	if err != nil {
		return nil, err
	}
	slog.Info("kie image task submitted", "task_id", taskID)

	urls, err := g.poll(ctx, taskID)
	if err != nil {
		return nil, err
	}

	results := make([][]byte, 0, len(urls))
	for _, u := range urls {
		data, err := g.download(ctx, u)
		if err != nil {
			return nil, fmt.Errorf("download image: %w", err)
		}
		results = append(results, data)
	}
	return results, nil
}

// ensurePublicURLs загружает локальные файлы в kie.ai и возвращает публичные URL.
func (g *KieImageGenerator) ensurePublicURLs(ctx context.Context, keys []string) ([]string, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	urls := make([]string, 0, len(keys))
	for _, key := range keys {
		data, err := g.storage.Download(ctx, key)
		if err != nil {
			slog.Warn("kie: skip ref image, download failed", "key", key, "err", err)
			continue
		}
		publicURL, err := g.uploadToKie(ctx, filepath.Base(key), data)
		if err != nil {
			slog.Warn("kie: skip ref image, upload failed", "key", key, "err", err)
			continue
		}
		urls = append(urls, publicURL)
	}
	return urls, nil
}

// uploadToKie загружает байты файла в kie.ai File Upload API и возвращает downloadUrl.
func (g *KieImageGenerator) uploadToKie(ctx context.Context, filename string, data []byte) (string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("uploadPath", "fungreet/ref-images")
	_ = mw.WriteField("fileName", filename)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := fw.Write(data); err != nil {
		return "", err
	}
	mw.Close()

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
	slog.Info("kie upload response", "status", resp.StatusCode, "body", string(raw))

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
		return "", fmt.Errorf("kie upload: empty downloadUrl, body: %s", string(raw))
	}
	return result.Data.DownloadURL, nil
}

func (g *KieImageGenerator) submit(ctx context.Context, prompt string, _ []string, callbackURL string) (string, error) {
	payload := map[string]any{
		"model": "z-image",
		"input": map[string]any{
			"prompt":       prompt,
			"aspect_ratio": "1:1",
		},
	}
	if callbackURL != "" {
		payload["callBackUrl"] = callbackURL
	}
	body, _ := json.Marshal(payload)

	slog.Info("kie submit request", "body", string(body))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, kieBaseURL+"/jobs/createTask", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+g.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("kie submit: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	slog.Info("kie submit response", "status", resp.StatusCode, "body", string(raw))

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			TaskID string `json:"taskId"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("kie submit decode: %w", err)
	}
	if result.Code != 200 {
		return "", fmt.Errorf("kie submit error %d: %s", result.Code, result.Msg)
	}
	return result.Data.TaskID, nil
}

func (g *KieImageGenerator) poll(ctx context.Context, taskID string) ([]string, error) {
	deadline := time.Now().Add(kieTimeout)
	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("kie image generation timeout after %s", kieTimeout)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(kiePollingInterval):
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			kieBaseURL+"/jobs/recordInfo?taskId="+taskID, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+g.apiKey)

		resp, err := g.client.Do(req)
		if err != nil {
			slog.Warn("kie poll error", "task_id", taskID, "err", err)
			continue
		}

		var result struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
			Data struct {
				State      string `json:"state"`
				FailMsg    string `json:"failMsg"`
				FailCode   any    `json:"failCode"`
				ResultJSON string `json:"resultJson"`
			} `json:"data"`
		}
		err = json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		if err != nil {
			slog.Warn("kie poll decode error", "task_id", taskID, "err", err)
			continue
		}

		slog.Info("kie poll", "task_id", taskID, "state", result.Data.State)

		switch result.Data.State {
		case "success":
			var rj struct {
				ResultURLs []string `json:"resultUrls"`
			}
			if err := json.Unmarshal([]byte(result.Data.ResultJSON), &rj); err != nil {
				return nil, fmt.Errorf("kie parse resultJson: %w", err)
			}
			if len(rj.ResultURLs) == 0 {
				return nil, fmt.Errorf("kie returned empty resultUrls")
			}
			return rj.ResultURLs, nil
		case "fail":
			slog.Error("kie task failed", "task_id", taskID, "fail_msg", result.Data.FailMsg, "fail_code", result.Data.FailCode)
			return nil, fmt.Errorf("kie generation failed: %s", result.Data.FailMsg)
		}
	}
}

func (g *KieImageGenerator) download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
