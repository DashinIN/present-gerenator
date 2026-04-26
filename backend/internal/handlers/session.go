package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/you/fungreet/internal/middleware"
	"github.com/you/fungreet/internal/models"
	"github.com/you/fungreet/internal/repository"
	"github.com/you/fungreet/internal/services"
)

type SessionHandler struct {
	sessionRepo *repository.SessionRepository
	genRepo     *repository.GenerationRepository
	storage     services.StorageService
}

func NewSessionHandler(sessionRepo *repository.SessionRepository, genRepo *repository.GenerationRepository, storage services.StorageService) *SessionHandler {
	return &SessionHandler{sessionRepo: sessionRepo, genRepo: genRepo, storage: storage}
}

func (h *SessionHandler) resolveKeys(ctx context.Context, keys []string) []string {
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

// SessionListResponse — список сессий
type SessionListResponse struct {
	Sessions []models.GenerationSession `json:"sessions"`
	Limit    int                        `json:"limit" example:"30"`
	Offset   int                        `json:"offset" example:"0"`
}

// SessionThreadResponse — сессия со всеми генерациями
type SessionThreadResponse struct {
	Session     models.GenerationSession  `json:"session"`
	Generations []models.GenerationRequest `json:"generations"`
}

// UpdateTitleRequest — запрос переименования сессии
type UpdateTitleRequest struct {
	Title string `json:"title" binding:"required,max=300" example:"Поздравление маме"`
}

// OkResponse — простой ответ об успехе
type OkResponse struct {
	Ok bool `json:"ok" example:"true"`
}

// List godoc
// @Summary      Список сессий
// @Description  Возвращает список диалоговых сессий (тредов) текущего пользователя, отсортированных по дате обновления.
// @Tags         sessions
// @Produce      json
// @Security     CookieAuth
// @Param        limit   query     int  false  "Лимит (макс. 100)"  default(30)
// @Param        offset  query     int  false  "Смещение"           default(0)
// @Success      200     {object}  SessionListResponse
// @Failure      401     {object}  ErrorResponse
// @Failure      500     {object}  ErrorResponse
// @Router       /sessions [get]
func (h *SessionHandler) List(c *gin.Context) {
	userID := middleware.GetUserID(c)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "30"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit > 100 {
		limit = 100
	}

	sessions, err := h.sessionRepo.ListByUser(c.Request.Context(), userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("internal_error", err.Error()))
		return
	}
	if sessions == nil {
		sessions = []models.GenerationSession{}
	}
	c.JSON(http.StatusOK, gin.H{"sessions": sessions, "limit": limit, "offset": offset})
}

// Get godoc
// @Summary      Получить сессию с генерациями
// @Description  Возвращает сессию и все генерации в ней (тред). URL изображений и аудио — рабочие ссылки для скачивания.
// @Tags         sessions
// @Produce      json
// @Security     CookieAuth
// @Param        id   path      string  true  "UUID сессии"
// @Success      200  {object}  SessionThreadResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      403  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /sessions/{id} [get]
func (h *SessionHandler) Get(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, apiError("invalid_param", "Invalid session ID"))
		return
	}

	session, err := h.sessionRepo.GetByID(c.Request.Context(), id)
	if err == repository.ErrNotFound {
		c.JSON(http.StatusNotFound, apiError("not_found", "Session not found"))
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("internal_error", err.Error()))
		return
	}
	if session.UserID != userID {
		c.JSON(http.StatusForbidden, apiError("forbidden", "Access denied"))
		return
	}

	gens, err := h.genRepo.ListBySession(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("internal_error", err.Error()))
		return
	}
	if gens == nil {
		gens = []models.GenerationRequest{}
	}
	for i := range gens {
		gens[i].ResultImages = h.resolveKeys(c.Request.Context(), gens[i].ResultImages)
		gens[i].ResultAudios = h.resolveKeys(c.Request.Context(), gens[i].ResultAudios)
		gens[i].InputPhotos = h.resolveKeys(c.Request.Context(), gens[i].InputPhotos)
	}

	c.JSON(http.StatusOK, gin.H{
		"session":     session,
		"generations": gens,
	})
}

// UpdateTitle godoc
// @Summary      Переименовать сессию
// @Description  Изменяет заголовок (название) сессии. Макс. 300 символов.
// @Tags         sessions
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Param        id    path      string              true  "UUID сессии"
// @Param        body  body      UpdateTitleRequest  true  "Новый заголовок"
// @Success      200   {object}  OkResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      403   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /sessions/{id} [patch]
func (h *SessionHandler) UpdateTitle(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, apiError("invalid_param", "Invalid session ID"))
		return
	}

	session, err := h.sessionRepo.GetByID(c.Request.Context(), id)
	if err == repository.ErrNotFound {
		c.JSON(http.StatusNotFound, apiError("not_found", "Session not found"))
		return
	}
	if err != nil || session.UserID != userID {
		c.JSON(http.StatusForbidden, apiError("forbidden", "Access denied"))
		return
	}

	var body struct {
		Title string `json:"title" binding:"required,max=300"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, apiError("invalid_param", "title required (max 300 chars)"))
		return
	}

	if err := h.sessionRepo.UpdateTitle(c.Request.Context(), id, body.Title); err != nil {
		c.JSON(http.StatusInternalServerError, apiError("internal_error", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
