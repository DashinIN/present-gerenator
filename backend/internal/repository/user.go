package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/you/fungreet/internal/models"
)

var ErrNotFound = errors.New("not found")

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) FindOrCreateByOAuth(ctx context.Context, profile models.OAuthProfile) (*models.User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Ищем существующий identity
	var userID int64
	err = tx.QueryRowContext(ctx,
		`SELECT user_id FROM user_identities WHERE provider = $1 AND provider_id = $2`,
		profile.Provider, profile.ProviderID,
	).Scan(&userID)

	if err == nil {
		// Identity найден — возвращаем пользователя
		user, err := r.findByIDTx(ctx, tx, userID)
		if err != nil {
			return nil, err
		}
		return user, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("query identity: %w", err)
	}

	// Пробуем найти user по email (слияние аккаунтов)
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM users WHERE email = $1`,
		profile.Email,
	).Scan(&userID)

	if errors.Is(err, sql.ErrNoRows) {
		// Создаём нового пользователя
		err = tx.QueryRowContext(ctx,
			`INSERT INTO users (email, display_name, avatar_url) VALUES ($1, $2, $3) RETURNING id`,
			profile.Email, profile.DisplayName, profile.AvatarURL,
		).Scan(&userID)
		if err != nil {
			return nil, fmt.Errorf("insert user: %w", err)
		}

		// Начисляем 50 стартовых кредитов
		_, err = tx.ExecContext(ctx,
			`INSERT INTO credit_transactions (user_id, amount, type, description)
			 VALUES ($1, 50, $2, 'Welcome bonus')`,
			userID, models.TxTypeInitialGrant,
		)
		if err != nil {
			return nil, fmt.Errorf("grant credits: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("query user by email: %w", err)
	}

	// Добавляем identity
	_, err = tx.ExecContext(ctx,
		`INSERT INTO user_identities (user_id, provider, provider_id, email) VALUES ($1, $2, $3, $4)`,
		userID, profile.Provider, profile.ProviderID, profile.Email,
	)
	if err != nil {
		return nil, fmt.Errorf("insert identity: %w", err)
	}

	user, err := r.findByIDTx(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	return user, tx.Commit()
}

func (r *UserRepository) FindByID(ctx context.Context, id int64) (*models.User, error) {
	var u models.User
	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, display_name, avatar_url, created_at FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.AvatarURL, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) GetIdentities(ctx context.Context, userID int64) ([]models.UserIdentity, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, provider, provider_id, email, created_at FROM user_identities WHERE user_id = $1`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.UserIdentity
	for rows.Next() {
		var i models.UserIdentity
		if err := rows.Scan(&i.ID, &i.UserID, &i.Provider, &i.ProviderID, &i.Email, &i.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, i)
	}
	return result, rows.Err()
}

func (r *UserRepository) findByIDTx(ctx context.Context, tx *sql.Tx, id int64) (*models.User, error) {
	var u models.User
	err := tx.QueryRowContext(ctx,
		`SELECT id, email, display_name, avatar_url, created_at FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.AvatarURL, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}
	return &u, nil
}
