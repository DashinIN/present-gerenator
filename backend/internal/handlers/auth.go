package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/you/fungreet/internal/middleware"
	"github.com/you/fungreet/internal/models"
	"github.com/you/fungreet/internal/repository"
	"github.com/you/fungreet/internal/services"
)

type AuthHandler struct {
	userRepo *repository.UserRepository
	jwt      *services.JWTService
}

func NewAuthHandler(userRepo *repository.UserRepository, jwt *services.JWTService) *AuthHandler {
	return &AuthHandler{userRepo: userRepo, jwt: jwt}
}

// DevLogin — мок для разработки. GET /api/auth/dev/login?user_id=1
// При отсутствии user_id создаёт нового тестового пользователя.
func (h *AuthHandler) DevLogin(c *gin.Context) {
	var userID int64

	if idStr := c.Query("user_id"); idStr != "" {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, apiError("invalid_param", "user_id must be integer"))
			return
		}
		userID = id
	} else {
		profile := models.OAuthProfile{
			Provider:    "dev",
			ProviderID:  fmt.Sprintf("dev-%d", time.Now().UnixNano()),
			Email:       fmt.Sprintf("dev-%d@example.com", time.Now().UnixMilli()),
			DisplayName: "Dev User",
			AvatarURL:   "",
		}
		user, err := h.userRepo.FindOrCreateByOAuth(c.Request.Context(), profile)
		if err != nil {
			c.JSON(http.StatusInternalServerError, apiError("internal_error", err.Error()))
			return
		}
		userID = user.ID
	}

	h.issueTokens(c, userID)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	cookie, err := c.Cookie("refresh_token")
	if err != nil {
		c.JSON(http.StatusUnauthorized, apiError("unauthorized", "Refresh token missing"))
		return
	}
	claims, err := h.jwt.Verify(cookie, services.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, apiError("unauthorized", "Invalid refresh token"))
		return
	}
	h.issueTokens(c, claims.UserID)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	clearCookie(c, "access_token", "/")
	clearCookie(c, "refresh_token", "/api/auth/refresh")
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID := middleware.GetUserID(c)
	user, err := h.userRepo.FindByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("internal_error", "User not found"))
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) issueTokens(c *gin.Context, userID int64) {
	access, err := h.jwt.Issue(userID, services.AccessToken, 15*time.Minute)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("internal_error", "Failed to issue token"))
		return
	}
	refresh, err := h.jwt.Issue(userID, services.RefreshToken, 30*24*time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("internal_error", "Failed to issue token"))
		return
	}

	setCookie(c, "access_token", access, "/", 15*60)
	setCookie(c, "refresh_token", refresh, "/api/auth/refresh", 30*24*60*60)

	c.JSON(http.StatusOK, gin.H{"user_id": userID})
}

func setCookie(c *gin.Context, name, value, path string, maxAge int) {
	c.SetCookie(name, value, maxAge, path, "", false, true)
}

func clearCookie(c *gin.Context, name, path string) {
	c.SetCookie(name, "", -1, path, "", false, true)
}

func apiError(code, msg string) gin.H {
	return gin.H{"error": gin.H{"code": code, "message": msg}}
}
