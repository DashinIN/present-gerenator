package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/you/fungreet/internal/models"
)

type SessionRepository struct {
	db *sql.DB
}

func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) Create(ctx context.Context, userID int64, title string) (*models.GenerationSession, error) {
	var s models.GenerationSession
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO generation_sessions (user_id, title) VALUES ($1, $2)
		 RETURNING id, user_id, title, created_at, updated_at`,
		userID, title,
	).Scan(&s.ID, &s.UserID, &s.Title, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SessionRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.GenerationSession, error) {
	var s models.GenerationSession
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, title, created_at, updated_at FROM generation_sessions WHERE id = $1`, id,
	).Scan(&s.ID, &s.UserID, &s.Title, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &s, err
}

func (r *SessionRepository) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]models.GenerationSession, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, title, created_at, updated_at
		 FROM generation_sessions WHERE user_id = $1
		 ORDER BY updated_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.GenerationSession
	for rows.Next() {
		var s models.GenerationSession
		if err := rows.Scan(&s.ID, &s.UserID, &s.Title, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// Touch обновляет updated_at — вызывается при каждой новой генерации в сессии.
func (r *SessionRepository) Touch(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE generation_sessions SET updated_at = NOW() WHERE id = $1`, id,
	)
	return err
}

func (r *SessionRepository) UpdateTitle(ctx context.Context, id uuid.UUID, title string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE generation_sessions SET title = $1 WHERE id = $2`, title, id,
	)
	return err
}
