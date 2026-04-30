package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDevLoginDisabledOutsideDevelopment(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewAuthHandler(nil, nil, nil, false)
	router := gin.New()
	router.GET("/api/auth/dev/login", handler.DevLogin)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/dev/login", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}
