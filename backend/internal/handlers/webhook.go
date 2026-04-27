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
	"github.com/you/fungreet/internal/models"
	"github.com/you/fungreet/internal/repository"
	"github.com/you/fungreet/internal/services"
	"github.com/you/fungreet/internal/worker"
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
	var payload struct {
		TaskID     string `json:"taskId"`
		State      string `json:"state"`
		ResultJSON string `json:"resultJson"`
		FailMsg    string `json:"failMsg"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	c.Status(http.StatusOK)
	go h.processKieCallback(payload.TaskID, payload.State, payload.ResultJSON, payload.FailMsg)
}

func (h *WebhookHandler) processKieCallback(taskID, state, resultJSON, failMsg string) {
	ctx := context.Background()

	meta, err := h.webhookStore.LookupTask(ctx, taskID)
	if err != nil {
		slog.Error("kie webhook: task not found", "task_id", taskID, "err", err)
		return
	}
	genID, _ := uuid.Parse(meta.GenID)

	if state != "success" {
		slog.Error("kie webhook: task failed", "task_id", taskID, "fail_msg", failMsg)
		h.failGeneration(ctx, genID, meta.UserID, fmt.Sprintf("image generation failed: %s", failMsg))
		return
	}

	var rj struct {
		ResultURLs []string `json:"resultUrls"`
	}
	if err := json.Unmarshal([]byte(resultJSON), &rj); err != nil || len(rj.ResultURLs) == 0 {
		slog.Error("kie webhook: empty resultUrls", "task_id", taskID)
		h.failGeneration(ctx, genID, meta.UserID, "image generation: empty result")
		return
	}

	keys := make([]string, 0, len(rj.ResultURLs))
	for i, u := range rj.ResultURLs {
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
	remaining, err := h.webhookStore.CompletePending(ctx, genID.String(), taskType)
	if err != nil {
		slog.Error("webhook: CompletePending failed", "err", err)
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
	_ = h.genRepo.UpdateStatus(ctx, genID, models.StatusFailed, reason)
	gen, err := h.genRepo.GetByID(ctx, genID)
	if err == nil {
		_ = h.billing.Refund(ctx, userID, gen.CreditsSpent, genID)
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
	return io.ReadAll(resp.Body)
}
