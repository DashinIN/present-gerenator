package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/you/fungreet/internal/models"
	"github.com/you/fungreet/internal/repository"
)

type BillingService struct {
	repo *repository.BillingRepository
}

func NewBillingService(repo *repository.BillingRepository) *BillingService {
	return &BillingService{repo: repo}
}

func (s *BillingService) GetActiveTariff(ctx context.Context) (*models.Tariff, error) {
	return s.repo.GetActiveTariff(ctx)
}

func (s *BillingService) CalculateCost(tariff *models.Tariff, images, songs, lyrics int) int {
	return tariff.PricePerImage*images + tariff.PricePerSong*songs + tariff.PricePerLyrics*lyrics
}

func (s *BillingService) GetBalance(ctx context.Context, userID int64) (int, error) {
	return s.repo.GetBalance(ctx, userID)
}

func (s *BillingService) Estimate(ctx context.Context, images, songs, lyrics int) (int, *models.Tariff, error) {
	tariff, err := s.repo.GetActiveTariff(ctx)
	if err != nil {
		return 0, nil, fmt.Errorf("get tariff: %w", err)
	}
	return s.CalculateCost(tariff, images, songs, lyrics), tariff, nil
}

func (s *BillingService) Charge(ctx context.Context, userID int64, amount int, refID uuid.UUID, desc string) error {
	return s.repo.Charge(ctx, userID, amount, models.TxTypeGenerationCharge, &refID, desc)
}

func (s *BillingService) Refund(ctx context.Context, userID int64, amount int, refID uuid.UUID) error {
	return s.repo.Refund(ctx, userID, amount, &refID)
}

func (s *BillingService) GetTransactions(ctx context.Context, userID int64, limit, offset int) ([]models.CreditTransaction, error) {
	return s.repo.GetTransactions(ctx, userID, limit, offset)
}
