package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID          int64     `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	AvatarURL   string    `json:"avatar_url"`
	CreatedAt   time.Time `json:"created_at"`
}

type UserIdentity struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	Provider   string    `json:"provider"` // google, vk, yandex, dev
	ProviderID string    `json:"provider_id"`
	Email      string    `json:"email"`
	CreatedAt  time.Time `json:"created_at"`
}

type OAuthProfile struct {
	Provider    string
	ProviderID  string
	Email       string
	DisplayName string
	AvatarURL   string
}

type Tariff struct {
	ID              int       `json:"id"`
	Name            string    `json:"name"`
	PricePerImage   int       `json:"price_per_image"`
	PricePerSong    int       `json:"price_per_song"`
	PricePerLyrics  int       `json:"price_per_lyrics"`
	IsActive        bool      `json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
}

type CreditTransaction struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	Amount      int       `json:"amount"` // положительное = начисление, отрицательное = списание
	Type        string    `json:"type"`
	ReferenceID *uuid.UUID `json:"reference_id,omitempty"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

const (
	TxTypeInitialGrant      = "initial_grant"
	TxTypeDailyGrant        = "daily_grant"
	TxTypeGenerationCharge  = "generation_charge"
	TxTypeGenerationRefund  = "generation_refund"
	TxTypePurchase          = "purchase"
)

type GenerationStatus string

const (
	StatusPending          GenerationStatus = "pending"
	StatusProcessingImages GenerationStatus = "processing_images"
	StatusProcessingAudio  GenerationStatus = "processing_audio"
	StatusCompleted        GenerationStatus = "completed"
	StatusFailed           GenerationStatus = "failed"
)

type GenerationSession struct {
	ID        uuid.UUID `json:"id"`
	UserID    int64     `json:"user_id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type GenerationRequest struct {
	ID            uuid.UUID        `json:"id"`
	UserID        int64            `json:"user_id"`
	SessionID     *uuid.UUID       `json:"session_id,omitempty"`
	ParentID      *uuid.UUID       `json:"parent_id,omitempty"`
	Status        GenerationStatus `json:"status"`
	ImagePrompt   string           `json:"image_prompt"`
	SongPrompt    string           `json:"song_prompt"`
	SongLyrics    string           `json:"song_lyrics"`
	SongStyle     string           `json:"song_style"`
	ImageCount    int              `json:"image_count"`
	SongCount     int              `json:"song_count"`
	InputPhotos   []string         `json:"input_photos"`
	InputAudioKey string           `json:"input_audio_key"`
	ResultImages  []string         `json:"result_images"`
	ResultAudios  []string         `json:"result_audios"`
	ErrorMessage  string           `json:"error_message,omitempty"`
	CreditsSpent  int              `json:"credits_spent"`
	TariffID      int              `json:"tariff_id"`
	CreatedAt     time.Time        `json:"created_at"`
	CompletedAt   *time.Time       `json:"completed_at,omitempty"`
}
