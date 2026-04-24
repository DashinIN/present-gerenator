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

// BalanceResponse — баланс пользователя в кредитах
type BalanceResponse struct {
	Balance int `json:"balance" example:"100"`
}

// EstimateResponse — предварительный расчёт стоимости
type EstimateResponse struct {
	Images        int `json:"images" example:"3"`
	Songs         int `json:"songs" example:"1"`
	Cost          int `json:"cost" example:"70"`
	PricePerImage int `json:"price_per_image" example:"15"`
	PricePerSong  int `json:"price_per_song" example:"25"`
}

// TransactionsResponse — список транзакций
type TransactionsResponse struct {
	Transactions []models.CreditTransaction `json:"transactions"`
	Limit        int                        `json:"limit" example:"20"`
	Offset       int                        `json:"offset" example:"0"`
}

// Balance godoc
// @Summary      Баланс кредитов
// @Description  Возвращает текущий баланс кредитов авторизованного пользователя.
// @Tags         billing
// @Produce      json
// @Security     CookieAuth
// @Success      200  {object}  BalanceResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /billing/balance [get]
func (h *BillingHandler) Balance(c *gin.Context) {
	userID := middleware.GetUserID(c)
	balance, err := h.billing.GetBalance(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("internal_error", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"balance": balance})
}

// Tariff godoc
// @Summary      Текущий тариф
// @Description  Возвращает активный тариф с ценами за изображение и песню.
// @Tags         billing
// @Produce      json
// @Security     CookieAuth
// @Success      200  {object}  models.Tariff
// @Failure      401  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /billing/tariff [get]
func (h *BillingHandler) Tariff(c *gin.Context) {
	tariff, err := h.billing.GetActiveTariff(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("internal_error", "No active tariff"))
		return
	}
	c.JSON(http.StatusOK, tariff)
}

// Estimate godoc
// @Summary      Предварительный расчёт стоимости
// @Description  Считает стоимость генерации до её запуска. Используется для отображения цены пользователю.
// @Tags         billing
// @Produce      json
// @Security     CookieAuth
// @Param        images  query     int  false  "Количество изображений (0-3)"  default(0)
// @Param        songs   query     int  false  "Количество песен (0-3)"         default(0)
// @Success      200     {object}  EstimateResponse
// @Failure      400     {object}  ErrorResponse
// @Failure      401     {object}  ErrorResponse
// @Failure      500     {object}  ErrorResponse
// @Router       /billing/estimate [get]
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

// Transactions godoc
// @Summary      История транзакций
// @Description  Возвращает постраничный список кредитных транзакций пользователя (начисления, списания, возвраты).
// @Tags         billing
// @Produce      json
// @Security     CookieAuth
// @Param        limit   query     int  false  "Лимит записей (макс. 100)"  default(20)
// @Param        offset  query     int  false  "Смещение"                    default(0)
// @Success      200     {object}  TransactionsResponse
// @Failure      401     {object}  ErrorResponse
// @Failure      500     {object}  ErrorResponse
// @Router       /billing/transactions [get]
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
