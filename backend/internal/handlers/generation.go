package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/you/fungreet/internal/middleware"
	"github.com/you/fungreet/internal/models"
	"github.com/you/fungreet/internal/repository"
	"github.com/you/fungreet/internal/services"
	"github.com/you/fungreet/internal/worker"
)

type GenerationHandler struct {
	genRepo     *repository.GenerationRepository
	sessionRepo *repository.SessionRepository
	billing     *services.BillingService
	storage     services.StorageService
	queue       *worker.Queue
}

func NewGenerationHandler(
	genRepo *repository.GenerationRepository,
	sessionRepo *repository.SessionRepository,
	billing *services.BillingService,
	storage services.StorageService,
	queue *worker.Queue,
) *GenerationHandler {
	return &GenerationHandler{
		genRepo:     genRepo,
		sessionRepo: sessionRepo,
		billing:     billing,
		storage:     storage,
		queue:       queue,
	}
}

// POST /api/generations
// Поля формы:
//   session_id     — UUID существующей сессии (опционально; если нет — создаём новую)
//   parent_id      — UUID предыдущей генерации в треде (опционально)
//   image_count    — 0..3 (default 3)
//   song_count     — 0..3 (default 1)
//   recipient_name, occasion, image_prompt, song_lyrics, song_style
//   photos[]       — файлы JPG/PNG до 10MB
//   audio          — файл MP3/WAV/M4A до 25MB
func (h *GenerationHandler) Create(c *gin.Context) {
	userID := middleware.GetUserID(c)

	imageCount, _ := strconv.Atoi(c.DefaultPostForm("image_count", "3"))
	songCount, _ := strconv.Atoi(c.DefaultPostForm("song_count", "1"))

	if imageCount < 0 || imageCount > 3 || songCount < 0 || songCount > 3 {
		c.JSON(http.StatusBadRequest, apiError("invalid_param", "image_count: 0-3, song_count: 0-3"))
		return
	}
	if imageCount == 0 && songCount == 0 {
		c.JSON(http.StatusBadRequest, apiError("invalid_param", "At least one of image_count or song_count must be > 0"))
		return
	}

	cost, tariff, err := h.billing.Estimate(c.Request.Context(), imageCount, songCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("internal_error", "Failed to get tariff"))
		return
	}

	balance, err := h.billing.GetBalance(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("internal_error", "Failed to check balance"))
		return
	}
	if balance < cost {
		c.JSON(http.StatusPaymentRequired, apiError("insufficient_credits",
			fmt.Sprintf("Need %d credits, have %d", cost, balance)))
		return
	}

	// Разбираем session_id и parent_id
	var sessionID *uuid.UUID
	var parentID *uuid.UUID

	if sidStr := c.PostForm("session_id"); sidStr != "" {
		sid, err := uuid.Parse(sidStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, apiError("invalid_param", "Invalid session_id"))
			return
		}
		// Проверяем что сессия принадлежит пользователю
		sess, err := h.sessionRepo.GetByID(c.Request.Context(), sid)
		if err == repository.ErrNotFound {
			c.JSON(http.StatusNotFound, apiError("not_found", "Session not found"))
			return
		}
		if err != nil || sess.UserID != userID {
			c.JSON(http.StatusForbidden, apiError("forbidden", "Access denied"))
			return
		}
		sessionID = &sid
	}

	if pidStr := c.PostForm("parent_id"); pidStr != "" {
		pid, err := uuid.Parse(pidStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, apiError("invalid_param", "Invalid parent_id"))
			return
		}
		parent, err := h.genRepo.GetByID(c.Request.Context(), pid)
		if err == repository.ErrNotFound {
			c.JSON(http.StatusNotFound, apiError("not_found", "Parent generation not found"))
			return
		}
		if err != nil || parent.UserID != userID {
			c.JSON(http.StatusForbidden, apiError("forbidden", "Access denied"))
			return
		}
		parentID = &pid
	}

	// Если сессии нет — создаём новую, используя prompt как заголовок
	if sessionID == nil {
		title := c.PostForm("recipient_name")
		if title == "" {
			title = c.PostForm("image_prompt")
		}
		if len(title) > 60 {
			title = title[:60]
		}
		if title == "" {
			title = "Новая генерация"
		}
		sess, err := h.sessionRepo.Create(c.Request.Context(), userID, title)
		if err != nil {
			c.JSON(http.StatusInternalServerError, apiError("internal_error", "Failed to create session"))
			return
		}
		sessionID = &sess.ID
	}

	// Загрузка фото
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, apiError("invalid_form", "Invalid multipart form"))
		return
	}

	photoKeys := []string{}
	if files := form.File["photos"]; len(files) > 0 {
		if len(files) > 3 {
			c.JSON(http.StatusBadRequest, apiError("invalid_param", "Max 3 photos"))
			return
		}
		for _, fh := range files {
			if fh.Size > 10<<20 {
				c.JSON(http.StatusBadRequest, apiError("invalid_param", "Photo max 10MB"))
				return
			}
			ext := strings.ToLower(filepath.Ext(fh.Filename))
			if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
				c.JSON(http.StatusBadRequest, apiError("invalid_param", "Photos must be JPG or PNG"))
				return
			}
			f, err := fh.Open()
			if err != nil {
				c.JSON(http.StatusInternalServerError, apiError("internal_error", "Failed to read photo"))
				return
			}
			defer f.Close()
			key := fmt.Sprintf("uploads/%d/%s%s", userID, uuid.New().String(), ext)
			if err := h.storage.Upload(c.Request.Context(), key, f, "image/jpeg"); err != nil {
				c.JSON(http.StatusInternalServerError, apiError("internal_error", "Failed to upload photo"))
				return
			}
			photoKeys = append(photoKeys, key)
		}
	}

	// Загрузка аудио
	var audioKey string
	if audioFiles := form.File["audio"]; len(audioFiles) == 1 {
		fh := audioFiles[0]
		if fh.Size > 25<<20 {
			c.JSON(http.StatusBadRequest, apiError("invalid_param", "Audio max 25MB"))
			return
		}
		ext := strings.ToLower(filepath.Ext(fh.Filename))
		if ext != ".mp3" && ext != ".wav" && ext != ".m4a" {
			c.JSON(http.StatusBadRequest, apiError("invalid_param", "Audio must be MP3, WAV or M4A"))
			return
		}
		f, err := fh.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, apiError("internal_error", "Failed to read audio"))
			return
		}
		defer f.Close()
		audioKey = fmt.Sprintf("uploads/%d/%s%s", userID, uuid.New().String(), ext)
		if err := h.storage.Upload(c.Request.Context(), audioKey, f, "audio/mpeg"); err != nil {
			c.JSON(http.StatusInternalServerError, apiError("internal_error", "Failed to upload audio"))
			return
		}
	}

	genID := uuid.New()

	if err := h.billing.Charge(c.Request.Context(), userID, cost, genID,
		fmt.Sprintf("%d images, %d songs", imageCount, songCount)); err != nil {
		if err == repository.ErrInsufficientCredits {
			c.JSON(http.StatusPaymentRequired, apiError("insufficient_credits", "Not enough credits"))
		} else {
			c.JSON(http.StatusInternalServerError, apiError("internal_error", "Failed to charge credits"))
		}
		return
	}

	gen, err := h.genRepo.Create(c.Request.Context(), repository.CreateGenerationParams{
		ID:            genID,
		UserID:        userID,
		SessionID:     sessionID,
		ParentID:      parentID,
		RecipientName: c.PostForm("recipient_name"),
		Occasion:      c.PostForm("occasion"),
		ImagePrompt:   c.PostForm("image_prompt"),
		SongLyrics:    c.PostForm("song_lyrics"),
		SongStyle:     c.PostForm("song_style"),
		ImageCount:    imageCount,
		SongCount:     songCount,
		InputPhotos:   photoKeys,
		InputAudioKey: audioKey,
		CreditsSpent:  cost,
		TariffID:      tariff.ID,
	})
	if err != nil {
		slog.Error("create generation failed", "err", err)
		_ = h.billing.Refund(c.Request.Context(), userID, cost, genID)
		c.JSON(http.StatusInternalServerError, apiError("internal_error", err.Error()))
		return
	}

	if err := h.queue.Push(c.Request.Context(), worker.Task{
		GenerationID: gen.ID,
		UserID:       userID,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, apiError("internal_error", "Failed to enqueue"))
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":         gen.ID,
		"session_id": gen.SessionID,
		"status":     gen.Status,
	})
}

// GET /api/generations
func (h *GenerationHandler) List(c *gin.Context) {
	userID := middleware.GetUserID(c)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit > 100 {
		limit = 100
	}

	gens, err := h.genRepo.ListByUser(c.Request.Context(), userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("internal_error", err.Error()))
		return
	}
	if gens == nil {
		gens = []models.GenerationRequest{}
	}
	for i := range gens {
		h.resolveGenURLs(c.Request.Context(), &gens[i])
	}
	c.JSON(http.StatusOK, gin.H{"generations": gens, "limit": limit, "offset": offset})
}

// GET /api/generations/:id
func (h *GenerationHandler) Get(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, apiError("invalid_param", "Invalid generation ID"))
		return
	}

	gen, err := h.genRepo.GetByID(c.Request.Context(), id)
	if err == repository.ErrNotFound {
		c.JSON(http.StatusNotFound, apiError("not_found", "Generation not found"))
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("internal_error", err.Error()))
		return
	}
	if gen.UserID != userID {
		c.JSON(http.StatusForbidden, apiError("forbidden", "Access denied"))
		return
	}
	h.resolveGenURLs(c.Request.Context(), gen)
	c.JSON(http.StatusOK, gen)
}

// GET /api/generations/:id/status
func (h *GenerationHandler) Status(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, apiError("invalid_param", "Invalid generation ID"))
		return
	}

	gen, err := h.genRepo.GetByID(c.Request.Context(), id)
	if err == repository.ErrNotFound {
		c.JSON(http.StatusNotFound, apiError("not_found", "Generation not found"))
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("internal_error", err.Error()))
		return
	}
	if gen.UserID != userID {
		c.JSON(http.StatusForbidden, apiError("forbidden", "Access denied"))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":            gen.ID,
		"status":        gen.Status,
		"error_message": gen.ErrorMessage,
		"completed_at":  gen.CompletedAt,
		"result_images": h.resolveKeys(c.Request.Context(), gen.ResultImages),
		"result_audios": h.resolveKeys(c.Request.Context(), gen.ResultAudios),
	})
}

// POST /api/uploads
func (h *GenerationHandler) Upload(c *gin.Context) {
	userID := middleware.GetUserID(c)
	file, fh, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, apiError("invalid_form", "Field 'file' required"))
		return
	}
	defer file.Close()

	if fh.Size > 25<<20 {
		c.JSON(http.StatusBadRequest, apiError("invalid_param", "Max file size 25MB"))
		return
	}

	ext := strings.ToLower(filepath.Ext(fh.Filename))
	allowed := map[string]string{
		".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png",
		".mp3": "audio/mpeg", ".wav": "audio/wav", ".m4a": "audio/mp4",
	}
	ct, ok := allowed[ext]
	if !ok {
		c.JSON(http.StatusBadRequest, apiError("invalid_param", "Unsupported file type"))
		return
	}

	key := fmt.Sprintf("uploads/%d/%s%s", userID, uuid.New().String(), ext)
	buf, _ := io.ReadAll(file)
	if err := h.storage.Upload(c.Request.Context(), key, bytes.NewReader(buf), ct); err != nil {
		c.JSON(http.StatusInternalServerError, apiError("internal_error", "Upload failed"))
		return
	}

	url, _ := h.storage.GetURL(c.Request.Context(), key)
	c.JSON(http.StatusOK, gin.H{"key": key, "url": url})
}

func (h *GenerationHandler) resolveGenURLs(ctx context.Context, gen *models.GenerationRequest) {
	gen.ResultImages = h.resolveKeys(ctx, gen.ResultImages)
	gen.ResultAudios = h.resolveKeys(ctx, gen.ResultAudios)
}

func (h *GenerationHandler) resolveKeys(ctx context.Context, keys []string) []string {
	urls := make([]string, len(keys))
	for i, key := range keys {
		if u, err := h.storage.GetURL(ctx, key); err == nil {
			urls[i] = u
		} else {
			urls[i] = key
		}
	}
	return urls
}
