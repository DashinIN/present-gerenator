package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/you/fungreet/internal/services"
)

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		slog.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency", time.Since(start).String(),
			"ip", c.ClientIP(),
		)
	}
}

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic recovered", "err", r)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{"code": "internal_error", "message": "Internal server error"},
				})
			}
		}()
		c.Next()
	}
}

const userIDKey = "user_id"

func AuthRequired(jwt *services.JWTService) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie("access_token")
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, apiErr("unauthorized", "Authentication required"))
			return
		}
		claims, err := jwt.Verify(cookie, services.AccessToken)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, apiErr("unauthorized", "Invalid or expired token"))
			return
		}
		c.Set(userIDKey, claims.UserID)
		c.Next()
	}
}

func GetUserID(c *gin.Context) int64 {
	v, _ := c.Get(userIDKey)
	id, _ := v.(int64)
	return id
}

func apiErr(code, msg string) gin.H {
	return gin.H{"error": gin.H{"code": code, "message": msg}}
}
