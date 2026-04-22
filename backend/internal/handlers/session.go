package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/you/fungreet/internal/middleware"
	"github.com/you/fungreet/internal/models"
	"github.com/you/fungreet/internal/repository"
)

type SessionHandler struct {
	sessionRepo *repository.SessionRepository
	genRepo     *repository.GenerationRepository
}

func NewSessionHandler(sessionRepo *repository.SessionRepository, genRepo *repository.GenerationRepository) *SessionHandler {
	return &SessionHandler{sessionRepo: sessionRepo, genRepo: genRepo}
}

// GET /api/sessions
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

// GET /api/sessions/:id  — сессия + все генерации в ней (тред)
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

	c.JSON(http.StatusOK, gin.H{
		"session":     session,
		"generations": gens,
	})
}

// PATCH /api/sessions/:id  — переименовать сессию
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
