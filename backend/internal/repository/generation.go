package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/you/fungreet/internal/models"
)

type GenerationRepository struct {
	db *sql.DB
}

func NewGenerationRepository(db *sql.DB) *GenerationRepository {
	return &GenerationRepository{db: db}
}

type CreateGenerationParams struct {
	ID            uuid.UUID  // совпадает с ID транзакции биллинга
	UserID        int64
	SessionID     *uuid.UUID
	ParentID      *uuid.UUID
	ImagePrompt   string
	SongPrompt    string
	SongLyrics    string
	SongStyle     string
	ImageCount    int
	SongCount     int
	InputPhotos   []string
	InputAudioKey string
	CreditsSpent  int
	TariffID      int
}

func (r *GenerationRepository) Create(ctx context.Context, p CreateGenerationParams) (*models.GenerationRequest, error) {
	var g models.GenerationRequest
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO generation_requests
		 (id, user_id, session_id, parent_id, image_prompt,
		  song_prompt, song_lyrics, song_style, image_count, song_count, input_photos, input_audio_key,
		  credits_spent, tariff_id)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		 RETURNING id, user_id, session_id, parent_id, status,
		           image_prompt, song_prompt, song_lyrics, song_style, image_count, song_count,
		           input_photos, input_audio_key, result_images, result_audios,
		           error_message, credits_spent, tariff_id, created_at, completed_at`,
		p.ID, p.UserID, p.SessionID, p.ParentID,
		p.ImagePrompt, p.SongPrompt, p.SongLyrics, p.SongStyle,
		p.ImageCount, p.SongCount, pq.Array(p.InputPhotos), p.InputAudioKey,
		p.CreditsSpent, p.TariffID,
	).Scan(scanGeneration(&g)...)
	if err != nil {
		return nil, fmt.Errorf("insert generation: %w", err)
	}
	return &g, nil
}

func (r *GenerationRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.GenerationRequest, error) {
	var g models.GenerationRequest
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, session_id, parent_id, status,
		        image_prompt, song_prompt, song_lyrics, song_style, image_count, song_count,
		        input_photos, input_audio_key, result_images, result_audios,
		        error_message, credits_spent, tariff_id, created_at, completed_at
		 FROM generation_requests WHERE id = $1`, id,
	).Scan(scanGeneration(&g)...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (r *GenerationRepository) ListBySession(ctx context.Context, sessionID uuid.UUID) ([]models.GenerationRequest, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, session_id, parent_id, status,
		        image_prompt, song_prompt, song_lyrics, song_style, image_count, song_count,
		        input_photos, input_audio_key, result_images, result_audios,
		        error_message, credits_spent, tariff_id, created_at, completed_at
		 FROM generation_requests WHERE session_id = $1
		 ORDER BY created_at ASC`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.GenerationRequest
	for rows.Next() {
		var g models.GenerationRequest
		if err := rows.Scan(scanGeneration(&g)...); err != nil {
			return nil, err
		}
		result = append(result, g)
	}
	return result, rows.Err()
}

func (r *GenerationRepository) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]models.GenerationRequest, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, session_id, parent_id, status,
		        image_prompt, song_prompt, song_lyrics, song_style, image_count, song_count,
		        input_photos, input_audio_key, result_images, result_audios,
		        error_message, credits_spent, tariff_id, created_at, completed_at
		 FROM generation_requests WHERE user_id = $1
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.GenerationRequest
	for rows.Next() {
		var g models.GenerationRequest
		if err := rows.Scan(scanGeneration(&g)...); err != nil {
			return nil, err
		}
		result = append(result, g)
	}
	return result, rows.Err()
}

func (r *GenerationRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.GenerationStatus, errMsg string) error {
	var completedAt *time.Time
	if status == models.StatusCompleted || status == models.StatusFailed {
		t := time.Now()
		completedAt = &t
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE generation_requests SET status=$1, error_message=$2, completed_at=$3 WHERE id=$4`,
		status, errMsg, completedAt, id,
	)
	return err
}

func (r *GenerationRepository) UpdateResults(ctx context.Context, id uuid.UUID, images, audios []string) error {
	t := time.Now()
	_, err := r.db.ExecContext(ctx,
		`UPDATE generation_requests
		 SET result_images=$1, result_audios=$2, status=$3, completed_at=$4
		 WHERE id=$5`,
		pq.Array(images), pq.Array(audios), models.StatusCompleted, t, id,
	)
	return err
}

// AppendAudios сохраняет частичные результаты аудио, не меняя статус генерации.
func (r *GenerationRepository) AppendAudios(ctx context.Context, id uuid.UUID, audios []string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE generation_requests SET result_audios=$1 WHERE id=$2`,
		pq.Array(audios), id,
	)
	return err
}

// AppendImages сохраняет результаты картинок, не меняя статус генерации.
func (r *GenerationRepository) AppendImages(ctx context.Context, id uuid.UUID, images []string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE generation_requests SET result_images=$1 WHERE id=$2`,
		pq.Array(images), id,
	)
	return err
}

func (r *GenerationRepository) IncrementRetry(ctx context.Context, id uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`UPDATE generation_requests SET retry_count = retry_count + 1 WHERE id=$1 RETURNING retry_count`,
		id,
	).Scan(&count)
	return count, err
}

func scanGeneration(g *models.GenerationRequest) []any {
	return []any{
		&g.ID, &g.UserID, &g.SessionID, &g.ParentID, &g.Status,
		&g.ImagePrompt, &g.SongPrompt, &g.SongLyrics, &g.SongStyle,
		&g.ImageCount, &g.SongCount,
		pq.Array(&g.InputPhotos), &g.InputAudioKey,
		pq.Array(&g.ResultImages), pq.Array(&g.ResultAudios),
		&g.ErrorMessage, &g.CreditsSpent, &g.TariffID, &g.CreatedAt, &g.CompletedAt,
	}
}
