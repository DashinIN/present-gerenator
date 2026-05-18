package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/you/funbaza/internal/middleware"
	"github.com/you/funbaza/internal/models"
	"github.com/you/funbaza/internal/repository"
	"github.com/you/funbaza/internal/services"
)

// DevLoginResponse — ответ на dev login
type DevLoginResponse struct {
	UserID int64 `json:"user_id" example:"42"`
}

// MeResponse — текущий пользователь
type MeResponse = models.User

// LogoutResponse — ответ при выходе
type LogoutResponse struct {
	Ok bool `json:"ok" example:"true"`
}

// ErrorResponse — стандартная ошибка API
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail — детали ошибки
type ErrorDetail struct {
	Code    string `json:"code" example:"unauthorized"`
	Message string `json:"message" example:"Refresh token missing"`
}

type AuthHandler struct {
	userRepo      *repository.UserRepository
	jwt           *services.JWTService
	billing       *services.BillingService
	googleOAuth   *services.GoogleOAuthService
	oauthState    *services.OAuthStateService
	allowDevLogin bool
}

func NewAuthHandler(userRepo *repository.UserRepository, jwt *services.JWTService, billing *services.BillingService, googleOAuth *services.GoogleOAuthService, oauthState *services.OAuthStateService, allowDevLogin bool) *AuthHandler {
	return &AuthHandler{
		userRepo:      userRepo,
		jwt:           jwt,
		billing:       billing,
		googleOAuth:   googleOAuth,
		oauthState:    oauthState,
		allowDevLogin: allowDevLogin,
	}
}

// DevLogin godoc
// @Summary      Dev login (только разработка)
// @Description  Создаёт тестового пользователя или логинится под существующим. Доступен только при APP_ENV=development.
// @Tags         auth
// @Produce      json
// @Param        user_id  query     int  false  "ID существующего пользователя (опционально)"
// @Success      200      {object}  DevLoginResponse
// @Failure      400      {object}  ErrorResponse
// @Failure      500      {object}  ErrorResponse
// @Router       /auth/dev/login [get]
func (h *AuthHandler) DevLogin(c *gin.Context) {
	if !h.allowDevLogin {
		c.JSON(http.StatusNotFound, apiError("not_found", "Not found"))
		return
	}

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

// GoogleLogin godoc
// @Summary      Войти через Google
// @Description  Перенаправляет пользователя на Google OAuth consent screen.
// @Tags         auth
// @Router       /auth/google/login [get]
func (h *AuthHandler) GoogleLogin(c *gin.Context) {
	if h.googleOAuth == nil || !h.googleOAuth.Enabled() {
		c.JSON(http.StatusNotFound, apiError("not_found", "Google auth is not configured"))
		return
	}

	state, err := h.oauthState.Encode(detectReturnTo(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("internal_error", "Failed to start Google auth"))
		return
	}

	c.Redirect(http.StatusFound, h.googleOAuth.AuthCodeURL(state))
}

// GoogleCallback godoc
// @Summary      Callback Google OAuth
// @Description  Завершает логин через Google, создаёт пользователя при первом входе и ставит JWT cookie.
// @Tags         auth
// @Router       /auth/google/callback [get]
func (h *AuthHandler) GoogleCallback(c *gin.Context) {
	if h.googleOAuth == nil || !h.googleOAuth.Enabled() {
		redirectWithError(c, "/", "google_not_configured")
		return
	}
	returnTo, err := h.oauthState.Decode(strings.TrimSpace(c.Query("state")))
	if err != nil {
		redirectWithError(c, "/", "invalid_oauth_state")
		return
	}
	returnTo = normalizeReturnTo(c, returnTo)
	if c.Query("error") != "" {
		redirectWithError(c, returnTo, "google_access_denied")
		return
	}

	code := strings.TrimSpace(c.Query("code"))
	if code == "" {
		redirectWithError(c, returnTo, "missing_oauth_code")
		return
	}

	profile, err := h.googleOAuth.ExchangeCode(c.Request.Context(), code)
	if err != nil {
		slog.Warn("google exchange failed", "err", err)
		redirectWithError(c, returnTo, "google_exchange_failed")
		return
	}

	user, err := h.userRepo.FindOrCreateByOAuth(c.Request.Context(), *profile)
	if err != nil {
		slog.Error("google login failed", "err", err, "email", profile.Email)
		redirectWithError(c, returnTo, "login_failed")
		return
	}

	if err := h.setAuthCookies(c, user.ID); err != nil {
		slog.Error("issue token failed", "err", err, "user_id", user.ID)
		redirectWithError(c, returnTo, "token_issue_failed")
		return
	}

	c.Redirect(http.StatusFound, returnTo)
}

// Refresh godoc
// @Summary      Обновить access token
// @Description  Берёт refresh_token из httpOnly cookie и выдаёт новую пару токенов.
// @Tags         auth
// @Produce      json
// @Success      200  {object}  DevLoginResponse
// @Failure      401  {object}  ErrorResponse
// @Router       /auth/refresh [post]
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

// Logout godoc
// @Summary      Выйти из аккаунта
// @Description  Очищает httpOnly cookie access_token и refresh_token.
// @Tags         auth
// @Produce      json
// @Security     CookieAuth
// @Success      200  {object}  LogoutResponse
// @Router       /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	h.clearCookie(c, "access_token", "/")
	h.clearCookie(c, "refresh_token", "/api/auth/refresh")
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// Me godoc
// @Summary      Текущий пользователь
// @Description  Возвращает профиль авторизованного пользователя.
// @Tags         user
// @Produce      json
// @Security     CookieAuth
// @Success      200  {object}  models.User
// @Failure      401  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /user/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if granted, err := h.billing.TryDailyGrant(c.Request.Context(), userID); err != nil {
		slog.Warn("daily grant failed", "user_id", userID, "err", err)
	} else if granted {
		slog.Info("daily grant issued", "user_id", userID)
	}
	user, err := h.userRepo.FindByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("internal_error", "User not found"))
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) issueTokens(c *gin.Context, userID int64) {
	if err := h.setAuthCookies(c, userID); err != nil {
		c.JSON(http.StatusInternalServerError, apiError("internal_error", "Failed to issue token"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"user_id": userID})
}

func (h *AuthHandler) setAuthCookies(c *gin.Context, userID int64) error {
	access, err := h.jwt.Issue(userID, services.AccessToken, 15*time.Minute)
	if err != nil {
		return err
	}
	refresh, err := h.jwt.Issue(userID, services.RefreshToken, 30*24*time.Hour)
	if err != nil {
		return err
	}

	h.setCookie(c, "access_token", access, "/", 15*60)
	h.setCookie(c, "refresh_token", refresh, "/api/auth/refresh", 30*24*60*60)
	return nil
}

func (h *AuthHandler) setCookie(c *gin.Context, name, value, path string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   !h.allowDevLogin,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *AuthHandler) clearCookie(c *gin.Context, name, path string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     path,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   !h.allowDevLogin,
		SameSite: http.SameSiteLaxMode,
	})
}

func apiError(code, msg string) gin.H {
	return gin.H{"error": gin.H{"code": code, "message": msg}}
}

func detectReturnTo(c *gin.Context) string {
	if origin := strings.TrimSpace(c.GetHeader("Origin")); origin != "" {
		return origin + "/"
	}
	if referer := strings.TrimSpace(c.GetHeader("Referer")); referer != "" {
		if u, err := url.Parse(referer); err == nil && u.Scheme != "" && u.Host != "" {
			return u.Scheme + "://" + u.Host + "/"
		}
	}
	return "/"
}

func normalizeReturnTo(c *gin.Context, returnTo string) string {
	target, err := url.Parse(returnTo)
	if err != nil || target.Host == "" {
		return requestBaseURL(c) + "/"
	}
	if !sameHost(target.Host, c.Request.Host) {
		return requestBaseURL(c) + "/"
	}
	return returnTo
}

func requestBaseURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if proto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); proto != "" {
		scheme = proto
	}
	return scheme + "://" + c.Request.Host
}

func sameHost(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func redirectWithError(c *gin.Context, returnTo, code string) {
	target, err := url.Parse(returnTo)
	if err != nil || returnTo == "" {
		target = &url.URL{Path: "/"}
	}
	q := target.Query()
	q.Set("auth_error", code)
	target.RawQuery = q.Encode()
	c.Redirect(http.StatusFound, target.String())
}
