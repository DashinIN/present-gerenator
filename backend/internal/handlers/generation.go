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
	songGen     services.SongGenerator
}

func NewGenerationHandler(
	genRepo *repository.GenerationRepository,
	sessionRepo *repository.SessionRepository,
	billing *services.BillingService,
	storage services.StorageService,
	queue *worker.Queue,
	songGen services.SongGenerator,
) *GenerationHandler {
	return &GenerationHandler{
		genRepo:     genRepo,
		sessionRepo: sessionRepo,
		billing:     billing,
		storage:     storage,
		queue:       queue,
		songGen:     songGen,
	}
}

// CreateGenerationResponse — ответ при создании генерации
type CreateGenerationResponse struct {
	ID        string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	SessionID string `json:"session_id" example:"550e8400-e29b-41d4-a716-446655440001"`
	Status    string `json:"status" example:"pending"`
}

// GenerationListResponse — список генераций
type GenerationListResponse struct {
	Generations []models.GenerationRequest `json:"generations"`
	Limit       int                        `json:"limit" example:"20"`
	Offset      int                        `json:"offset" example:"0"`
}

// GenerationStatusResponse — статус генерации
type GenerationStatusResponse struct {
	ID           string     `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Status       string     `json:"status" example:"completed"`
	ErrorMessage string     `json:"error_message,omitempty"`
	CompletedAt  *string    `json:"completed_at,omitempty"`
	ResultImages []string   `json:"result_images" example:"[\"http://localhost:8080/api/files/uploads/1/abc.jpg\"]"`
	ResultAudios []string   `json:"result_audios" example:"[\"http://localhost:8080/api/files/uploads/1/abc.mp3\"]"`
}

// UploadResponse — ответ при загрузке файла
type UploadResponse struct {
	Key string `json:"key" example:"uploads/42/uuid.jpg"`
	URL string `json:"url" example:"http://localhost:8080/api/files/uploads/42/uuid.jpg"`
}

// Create godoc
// @Summary      Создать генерацию
// @Description  Запускает задачу генерации изображений и/или песни. Списывает кредиты. Возвращает ID задачи для последующего опроса статуса через GET /generations/:id/status.
// @Tags         generations
// @Accept       multipart/form-data
// @Produce      json
// @Security     CookieAuth
// @Param        session_id      formData  string  false  "UUID существующей сессии (если нет — создаётся новая)"
// @Param        parent_id       formData  string  false  "UUID родительской генерации в треде"
// @Param        image_count     formData  int     false  "Количество изображений (0-3)"     default(3)
// @Param        song_count      formData  int     false  "Количество песен (0-3)"            default(1)
// @Param        image_prompt    formData  string  false  "Текстовое описание для генерации изображений"
// @Param        song_prompt     formData  string  false  "Промт для генерации текста песни (если song_lyrics не задан)"
// @Param        song_lyrics     formData  string  false  "Текст песни (если задан — song_prompt игнорируется)"
// @Param        song_style      formData  string  false  "Стиль песни (pop, jazz, rock и т.д.)"
// @Param        photos[]        formData  file    false  "Фото пользователя JPG/PNG до 10MB (макс. 3)"
// @Param        audio           formData  file    false  "Аудио MP3/WAV/M4A до 25MB"
// @Success      201             {object}  CreateGenerationResponse
// @Failure      400             {object}  ErrorResponse
// @Failure      401             {object}  ErrorResponse
// @Failure      402             {object}  ErrorResponse  "Недостаточно кредитов"
// @Failure      403             {object}  ErrorResponse
// @Failure      404             {object}  ErrorResponse
// @Failure      500             {object}  ErrorResponse
// @Router       /generations [post]
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

	lyricsCount := 0
	if songCount > 0 && c.PostForm("song_prompt") != "" && c.PostForm("song_lyrics") == "" {
		lyricsCount = 1
	}

	cost, tariff, err := h.billing.Estimate(c.Request.Context(), imageCount, songCount, lyricsCount)
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
		title := c.PostForm("image_prompt")
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
		ImagePrompt:   c.PostForm("image_prompt"),
		SongPrompt:    c.PostForm("song_prompt"),
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

// List godoc
// @Summary      Список генераций
// @Description  Возвращает постраничный список всех генераций текущего пользователя.
// @Tags         generations
// @Produce      json
// @Security     CookieAuth
// @Param        limit   query     int  false  "Лимит (макс. 100)"  default(20)
// @Param        offset  query     int  false  "Смещение"           default(0)
// @Success      200     {object}  GenerationListResponse
// @Failure      401     {object}  ErrorResponse
// @Failure      500     {object}  ErrorResponse
// @Router       /generations [get]
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

// Get godoc
// @Summary      Получить генерацию
// @Description  Возвращает полные данные генерации включая URL результирующих файлов.
// @Tags         generations
// @Produce      json
// @Security     CookieAuth
// @Param        id   path      string  true  "UUID генерации"
// @Success      200  {object}  models.GenerationRequest
// @Failure      400  {object}  ErrorResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      403  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /generations/{id} [get]
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

// Status godoc
// @Summary      Статус генерации (polling)
// @Description  Возвращает текущий статус задачи генерации. Используется для polling-а: опрашивайте каждые 2-5 секунд пока status != completed | failed.
// @Tags         generations
// @Produce      json
// @Security     CookieAuth
// @Param        id   path      string  true  "UUID генерации"
// @Success      200  {object}  GenerationStatusResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      403  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /generations/{id}/status [get]
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

// Upload godoc
// @Summary      Загрузить файл
// @Description  Загружает файл (фото или аудио) в хранилище. Возвращает key для последующего использования при создании генерации. Поддерживаемые форматы: JPG, PNG, MP3, WAV, M4A. Макс. размер: 25MB.
// @Tags         uploads
// @Accept       multipart/form-data
// @Produce      json
// @Security     CookieAuth
// @Param        file  formData  file    true  "Файл для загрузки"
// @Success      200   {object}  UploadResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /uploads [post]
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

// LyricsRequest — запрос генерации текста песни
type LyricsRequest struct {
	Prompt string `json:"prompt" binding:"required"`
}

// LyricsResponse — сгенерированный текст песни
type LyricsResponse struct {
	Text  string `json:"text" example:"Куплет 1...\nПрипев..."`
	Title string `json:"title" example:"Поздравление"`
}

// GenerateLyrics godoc
// @Summary      Сгенерировать текст песни
// @Description  Генерирует текст и заголовок песни по промту через Suno AI. Требует SUNO_API_KEY.
// @Tags         generations
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Param        body  body      LyricsRequest  true  "Промт"
// @Success      200   {object}  LyricsResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      503   {object}  ErrorResponse  "Генератор текста недоступен"
// @Router       /generations/lyrics [post]
func (h *GenerationHandler) GenerateLyrics(c *gin.Context) {
	lg, ok := h.songGen.(services.LyricsGenerator)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, apiError("lyrics_unavailable", "Lyrics generator not configured"))
		return
	}

	var req LyricsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError("invalid_param", "prompt required"))
		return
	}

	userID := middleware.GetUserID(c)

	tariff, err := h.billing.GetActiveTariff(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("internal_error", "Failed to get tariff"))
		return
	}

	cost := tariff.PricePerLyrics
	if cost > 0 {
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
		if err := h.billing.Charge(c.Request.Context(), userID, cost, uuid.New(), "Lyrics generation"); err != nil {
			c.JSON(http.StatusInternalServerError, apiError("internal_error", "Failed to charge credits"))
			return
		}
	}

	text, title, err := lg.GenerateLyrics(c.Request.Context(), req.Prompt)
	if err != nil {
		if cost > 0 {
			_ = h.billing.Refund(c.Request.Context(), userID, cost, uuid.New())
		}
		slog.Error("lyrics generation failed", "err", err)
		c.JSON(http.StatusInternalServerError, apiError("generation_failed", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"text": text, "title": title})
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
