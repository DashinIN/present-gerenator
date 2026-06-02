package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/you/funbaza/internal/models"
)

var ErrInsufficientCredits = errors.New("insufficient credits")

type BillingRepository struct {
	db *sql.DB
}

func NewBillingRepository(db *sql.DB) *BillingRepository {
	return &BillingRepository{db: db}
}

func (r *BillingRepository) GetActiveTariff(ctx context.Context) (*models.Tariff, error) {
	var t models.Tariff
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, price_per_image, price_per_song, price_per_lyrics, is_active, created_at
		 FROM tariffs WHERE is_active = true LIMIT 1`,
	).Scan(&t.ID, &t.Name, &t.PricePerImage, &t.PricePerSong, &t.PricePerLyrics, &t.IsActive, &t.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *BillingRepository) GetBalance(ctx context.Context, userID int64) (int, error) {
	var balance sql.NullInt64
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM credit_transactions WHERE user_id = $1`,
		userID,
	).Scan(&balance)
	if err != nil {
		return 0, err
	}
	return int(balance.Int64), nil
}

// Charge атомарно проверяет баланс и списывает кредиты.
func (r *BillingRepository) Charge(ctx context.Context, userID int64, amount int, txType string, refID *uuid.UUID, desc string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if err := lockUserLedger(ctx, tx, userID); err != nil {
		return err
	}

	var balance sql.NullInt64
	err = tx.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM credit_transactions WHERE user_id = $1`,
		userID,
	).Scan(&balance)
	if err != nil {
		return fmt.Errorf("check balance: %w", err)
	}
	if int(balance.Int64) < amount {
		return ErrInsufficientCredits
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO credit_transactions (user_id, amount, type, reference_id, description)
		 VALUES ($1, $2, $3, $4, $5)`,
		userID, -amount, txType, refID, desc,
	)
	if err != nil {
		return fmt.Errorf("insert transaction: %w", err)
	}
	return tx.Commit()
}

func (r *BillingRepository) Refund(ctx context.Context, userID int64, amount int, refID *uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if err := lockUserLedger(ctx, tx, userID); err != nil {
		return err
	}

	if refID != nil {
		var exists bool
		if err := tx.QueryRowContext(ctx,
			`SELECT EXISTS (
				SELECT 1 FROM credit_transactions
				WHERE type = $1 AND reference_id = $2
			)`,
			models.TxTypeGenerationRefund, refID,
		).Scan(&exists); err != nil {
			return fmt.Errorf("check refund: %w", err)
		}
		if exists {
			return tx.Commit()
		}
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO credit_transactions (user_id, amount, type, reference_id, description)
		 VALUES ($1, $2, $3, $4, 'Refund for failed generation')
		 ON CONFLICT (type, reference_id) WHERE reference_id IS NOT NULL DO NOTHING`,
		userID, amount, models.TxTypeGenerationRefund, refID,
	)
	if err != nil {
		return fmt.Errorf("insert refund: %w", err)
	}
	return tx.Commit()
}

// TryDailyGrant atomically tops the balance up to cap if the user has not
// received a daily grant during the last 24 hours.
// Returns true when credits were inserted because the balance was below cap.
func (r *BillingRepository) TryDailyGrant(ctx context.Context, userID int64, cap int) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if err := lockUserLedger(ctx, tx, userID); err != nil {
		return false, err
	}

	// Use a rolling 24-hour window, not CURRENT_DATE. CURRENT_DATE would allow
	// another grant right after midnight even when less than a day has passed.
	var count int
	if err = tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM credit_transactions
		 WHERE user_id = $1
		   AND type = $2
		   AND created_at >= NOW() - INTERVAL '24 hours'`,
		userID, models.TxTypeDailyGrant,
	).Scan(&count); err != nil {
		return false, fmt.Errorf("check daily grant: %w", err)
	}
	if count > 0 {
		return false, nil
	}

	// Calculate current balance.
	var balance int64
	if err = tx.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM credit_transactions WHERE user_id = $1`,
		userID,
	).Scan(&balance); err != nil {
		return false, fmt.Errorf("get balance: %w", err)
	}

	topup := int64(cap) - balance
	if topup <= 0 {
		return false, nil
	}

	if _, err = tx.ExecContext(ctx,
		`INSERT INTO credit_transactions (user_id, amount, type, description)
		 VALUES ($1, $2, $3, 'Daily top-up to limit')`,
		userID, topup, models.TxTypeDailyGrant,
	); err != nil {
		return false, fmt.Errorf("insert daily grant: %w", err)
	}
	return true, tx.Commit()
}

func lockUserLedger(ctx context.Context, tx *sql.Tx, userID int64) error {
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, userID); err != nil {
		return fmt.Errorf("lock user ledger: %w", err)
	}
	return nil
}

func (r *BillingRepository) GetTransactions(ctx context.Context, userID int64, limit, offset int) ([]models.CreditTransaction, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, amount, type, reference_id, description, created_at
		 FROM credit_transactions WHERE user_id = $1
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.CreditTransaction
	for rows.Next() {
		var t models.CreditTransaction
		var refID sql.NullString
		if err := rows.Scan(&t.ID, &t.UserID, &t.Amount, &t.Type, &refID, &t.Description, &t.CreatedAt); err != nil {
			return nil, err
		}
		if refID.Valid {
			parsed, _ := uuid.Parse(refID.String)
			t.ReferenceID = &parsed
		}
		result = append(result, t)
	}
	return result, rows.Err()
}
