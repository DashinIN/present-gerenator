package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/you/funbaza/internal/repository"
	"github.com/you/funbaza/internal/services"
	"github.com/you/funbaza/internal/worker"
)

type WebhookHandler struct {
	webhookStore *worker.WebhookStore
	genRepo      *repository.GenerationRepository
	sessionRepo  *repository.SessionRepository
	billing      *services.BillingService
	storage      services.StorageService
}

func NewWebhookHandler(
	webhookStore *worker.WebhookStore,
	genRepo *repository.GenerationRepository,
	sessionRepo *repository.SessionRepository,
	billing *services.BillingService,
	storage services.StorageService,
) *WebhookHandler {
	return &WebhookHandler{
		webhookStore: webhookStore,
		genRepo:      genRepo,
		sessionRepo:  sessionRepo,
		billing:      billing,
		storage:      storage,
	}
}

// KieCallback обрабатывает вебхук от kie.ai после генерации картинки.
func (h *WebhookHandler) KieCallback(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	c.Status(http.StatusOK)
	go h.processKieCallback(body)
}

func (h *WebhookHandler) processKieCallback(body []byte) {
	ctx := context.Background()

	payload, err := parseKieCallback(body)
	if err != nil {
		slog.Error("kie webhook: decode error", "err", err)
		return
	}

	meta, err := h.webhookStore.LookupTask(ctx, payload.TaskID)
	if err != nil {
		slog.Error("kie webhook: task not found", "task_id", payload.TaskID, "err", err)
		return
	}
	genID, _ := uuid.Parse(meta.GenID)

	if payload.State != "success" {
		slog.Error("kie webhook: task failed", "task_id", payload.TaskID, "state", payload.State, "fail_msg", payload.FailMsg)
		h.failGeneration(ctx, genID, meta.UserID, fmt.Sprintf("image generation failed: %s", payload.FailMsg))
		return
	}

	if len(payload.ResultURLs) == 0 {
		slog.Error("kie webhook: empty resultUrls", "task_id", payload.TaskID)
		h.failGeneration(ctx, genID, meta.UserID, "image generation: empty result")
		return
	}

	keys := make([]string, 0, len(payload.ResultURLs))
	for i, u := range payload.ResultURLs {
		data, err := downloadURL(ctx, u)
		if err != nil {
			slog.Error("kie webhook: download failed", "url", u, "err", err)
			h.failGeneration(ctx, genID, meta.UserID, "image download failed")
			return
		}
		key := fmt.Sprintf("results/%s/image_%d.png", genID, i)
		if err := h.storage.Upload(ctx, key, bytes.NewReader(data), "image/png"); err != nil {
			slog.Error("kie webhook: upload failed", "key", key, "err", err)
			h.failGeneration(ctx, genID, meta.UserID, "image upload failed")
			return
		}
		keys = append(keys, key)
	}

	if err := h.genRepo.AppendImages(ctx, genID, keys); err != nil {
		slog.Error("kie webhook: AppendImages failed", "err", err)
		return
	}
	slog.Info("kie webhook: images saved", "generation_id", genID, "count", len(keys))
	h.tryComplete(ctx, genID, "image")
}

type kieCallbackData struct {
	TaskID     string
	State      string
	ResultURLs []string
	FailMsg    string
}

func parseKieCallback(body []byte) (*kieCallbackData, error) {
	var payload struct {
		Code       int             `json:"code"`
		Msg        string          `json:"msg"`
		TaskID     string          `json:"taskId"`
		TaskIDAlt  string          `json:"task_id"`
		State      string          `json:"state"`
		ResultJSON string          `json:"resultJson"`
		FailMsg    string          `json:"failMsg"`
		Data       json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	result := &kieCallbackData{
		TaskID:     firstNonEmpty(payload.TaskID, payload.TaskIDAlt),
		State:      payload.State,
		ResultURLs: parseKieResultURLs(payload.ResultJSON),
		FailMsg:    firstNonEmpty(payload.FailMsg, payload.Msg),
	}

	if len(payload.Data) > 0 {
		var data struct {
			TaskID     string `json:"taskId"`
			TaskIDAlt  string `json:"task_id"`
			State      string `json:"state"`
			Status     string `json:"status"`
			ResultJSON string `json:"resultJson"`
			FailMsg    string `json:"failMsg"`
			ErrorMsg   string `json:"errorMessage"`
			Info       struct {
				ResultURLs []string `json:"result_urls"`
				ResultUrls []string `json:"resultUrls"`
			} `json:"info"`
			Response struct {
				ResultURLs []string `json:"resultUrls"`
			} `json:"response"`
		}
		if err := json.Unmarshal(payload.Data, &data); err == nil {
			result.TaskID = firstNonEmpty(result.TaskID, data.TaskID, data.TaskIDAlt)
			result.State = firstNonEmpty(result.State, data.State, normalizeKieStatus(data.Status))
			result.FailMsg = firstNonEmpty(result.FailMsg, data.FailMsg, data.ErrorMsg)
			if urls := parseKieResultURLs(data.ResultJSON); len(urls) > 0 {
				result.ResultURLs = urls
			} else if len(data.Info.ResultURLs) > 0 {
				result.ResultURLs = data.Info.ResultURLs
			} else if len(data.Info.ResultUrls) > 0 {
				result.ResultURLs = data.Info.ResultUrls
			} else if len(data.Response.ResultURLs) > 0 {
				result.ResultURLs = data.Response.ResultURLs
			}
		}
	}

	if result.State == "" && payload.Code != 0 {
		if payload.Code == http.StatusOK {
			result.State = "success"
		} else {
			result.State = "fail"
		}
	}
	if result.TaskID == "" {
		return nil, fmt.Errorf("missing task id")
	}
	return result, nil
}

func parseKieResultURLs(resultJSON string) []string {
	if resultJSON == "" {
		return nil
	}
	var rj struct {
		ResultURLs []string `json:"resultUrls"`
		ResultUrls []string `json:"result_urls"`
		ImageURLs  []string `json:"image_urls"`
		Images     []string `json:"images"`
	}
	if err := json.Unmarshal([]byte(resultJSON), &rj); err != nil {
		return nil
	}
	switch {
	case len(rj.ResultURLs) > 0:
		return rj.ResultURLs
	case len(rj.ResultUrls) > 0:
		return rj.ResultUrls
	case len(rj.ImageURLs) > 0:
		return rj.ImageURLs
	default:
		return rj.Images
	}
}

func normalizeKieStatus(status string) string {
	switch status {
	case "SUCCESS":
		return "success"
	case "CREATE_TASK_FAILED", "GENERATE_FAILED", "GENERATE_AUDIO_FAILED":
		return "fail"
	default:
		return status
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// SunoCallback обрабатывает вебхук от sunoapi.org после генерации песни.
func (h *WebhookHandler) SunoCallback(c *gin.Context) {
	body, _ := io.ReadAll(c.Request.Body)
	c.Status(http.StatusOK)
	go h.processSunoCallback(body)
}

func (h *WebhookHandler) processSunoCallback(body []byte) {
	ctx := context.Background()

	var payload struct {
		TaskID string `json:"taskId"`
		Status string `json:"status"`
		Data   struct {
			Response struct {
				SunoData []struct {
					AudioURL string `json:"audioUrl"`
				} `json:"sunoData"`
			} `json:"response"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		slog.Error("suno webhook: decode error", "err", err)
		return
	}

	// Suno шлёт callback на каждый статус — нас интересует только финальный SUCCESS
	if payload.Status != "SUCCESS" {
		slog.Info("suno webhook: intermediate status", "task_id", payload.TaskID, "status", payload.Status)
		return
	}

	meta, err := h.webhookStore.LookupTask(ctx, payload.TaskID)
	if err != nil {
		slog.Error("suno webhook: task not found", "task_id", payload.TaskID, "err", err)
		return
	}
	genID, _ := uuid.Parse(meta.GenID)

	clips := payload.Data.Response.SunoData
	if len(clips) == 0 {
		h.failGeneration(ctx, genID, meta.UserID, "song generation: empty result")
		return
	}

	keys := make([]string, 0, len(clips))
	for i, clip := range clips {
		if clip.AudioURL == "" {
			continue
		}
		data, err := downloadURL(ctx, clip.AudioURL)
		if err != nil {
			slog.Error("suno webhook: download failed", "url", clip.AudioURL, "err", err)
			h.failGeneration(ctx, genID, meta.UserID, "audio download failed")
			return
		}
		key := fmt.Sprintf("results/%s/song_%d.mp3", genID, i)
		if err := h.storage.Upload(ctx, key, bytes.NewReader(data), "audio/mpeg"); err != nil {
			slog.Error("suno webhook: upload failed", "key", key, "err", err)
			h.failGeneration(ctx, genID, meta.UserID, "audio upload failed")
			return
		}
		keys = append(keys, key)
	}

	if err := h.genRepo.AppendAudios(ctx, genID, keys); err != nil {
		slog.Error("suno webhook: AppendAudios failed", "err", err)
		return
	}
	slog.Info("suno webhook: audios saved", "generation_id", genID, "count", len(keys))
	h.tryComplete(ctx, genID, "song")
}

func (h *WebhookHandler) tryComplete(ctx context.Context, genID uuid.UUID, taskType string) {
	removed, remaining, err := h.webhookStore.CompletePending(ctx, genID.String(), taskType)
	if err != nil {
		slog.Error("webhook: CompletePending failed", "err", err)
		return
	}
	if !removed {
		slog.Info("webhook: duplicate or stale callback ignored", "generation_id", genID, "task_type", taskType)
		return
	}
	if remaining > 0 {
		slog.Info("webhook: waiting for more results", "generation_id", genID, "remaining", remaining)
		return
	}

	gen, err := h.genRepo.GetByID(ctx, genID)
	if err != nil {
		slog.Error("webhook: GetByID failed", "err", err)
		return
	}

	if err := h.genRepo.UpdateResults(ctx, genID, gen.ResultImages, gen.ResultAudios); err != nil {
		slog.Error("webhook: UpdateResults failed", "err", err)
		return
	}

	if gen.SessionID != nil {
		_ = h.sessionRepo.Touch(ctx, *gen.SessionID)
	}

	slog.Info("webhook: generation completed", "generation_id", genID,
		"images", len(gen.ResultImages), "songs", len(gen.ResultAudios))
}

func (h *WebhookHandler) failGeneration(ctx context.Context, genID uuid.UUID, userID int64, reason string) {
	marked, err := h.genRepo.MarkFailedIfActive(ctx, genID, reason)
	if err != nil {
		slog.Error("webhook: MarkFailedIfActive failed", "generation_id", genID, "err", err)
		return
	}
	if !marked {
		slog.Info("webhook: failure ignored for terminal generation", "generation_id", genID)
		return
	}
	if gen, err := h.genRepo.GetByID(ctx, genID); err == nil {
		if err := h.billing.Refund(ctx, userID, gen.CreditsSpent, genID); err != nil {
			slog.Error("webhook: refund failed", "generation_id", genID, "err", err)
		}
	}
}

var downloadClient = &http.Client{Timeout: 60 * time.Second}

func downloadURL(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := downloadClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download http %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
