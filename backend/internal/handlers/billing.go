package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/you/fungreet/internal/middleware"
	"github.com/you/fungreet/internal/models"
	"github.com/you/fungreet/internal/services"
)

type BillingHandler struct {
	billing *services.BillingService
}

func NewBillingHandler(billing *services.BillingService) *BillingHandler {
	return &BillingHandler{billing: billing}
}

// GET /api/billing/balance
func (h *BillingHandler) Balance(c *gin.Context) {
	userID := middleware.GetUserID(c)
	balance, err := h.billing.GetBalance(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("internal_error", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"balance": balance})
}

// GET /api/billing/tariff
func (h *BillingHandler) Tariff(c *gin.Context) {
	tariff, err := h.billing.GetActiveTariff(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("internal_error", "No active tariff"))
		return
	}
	c.JSON(http.StatusOK, tariff)
}

// GET /api/billing/estimate?images=3&songs=1
func (h *BillingHandler) Estimate(c *gin.Context) {
	images, _ := strconv.Atoi(c.DefaultQuery("images", "0"))
	songs, _ := strconv.Atoi(c.DefaultQuery("songs", "0"))

	if images < 0 || images > 3 || songs < 0 || songs > 3 {
		c.JSON(http.StatusBadRequest, apiError("invalid_param", "images: 0-3, songs: 0-3"))
		return
	}
	if images == 0 && songs == 0 {
		c.JSON(http.StatusBadRequest, apiError("invalid_param", "At least one of images or songs must be > 0"))
		return
	}

	cost, tariff, err := h.billing.Estimate(c.Request.Context(), images, songs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("internal_error", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"images":         images,
		"songs":          songs,
		"cost":           cost,
		"price_per_image": tariff.PricePerImage,
		"price_per_song":  tariff.PricePerSong,
	})
}

// GET /api/billing/transactions?limit=20&offset=0
func (h *BillingHandler) Transactions(c *gin.Context) {
	userID := middleware.GetUserID(c)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit > 100 {
		limit = 100
	}

	txs, err := h.billing.GetTransactions(c.Request.Context(), userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("internal_error", err.Error()))
		return
	}
	if txs == nil {
		txs = []models.CreditTransaction{}
	}
	c.JSON(http.StatusOK, gin.H{"transactions": txs, "limit": limit, "offset": offset})
}
