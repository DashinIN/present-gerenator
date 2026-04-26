package services

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"time"
)

//go:embed assets/example.png
var placeholderPNG []byte

//go:embed assets/example.mp3
var placeholderMP3 []byte

// ImageGenerator генерирует картинки.
type ImageGenerator interface {
	Generate(ctx context.Context, prompt string, refImages []string, count int) ([][]byte, error)
}

// SongGenerator генерирует песни.
type SongGenerator interface {
	Generate(ctx context.Context, lyrics, style string, count int) ([][]byte, error)
}

// LyricsGenerator — опциональное расширение SongGenerator: генерирует текст и заголовок песни по промту.
type LyricsGenerator interface {
	GenerateLyrics(ctx context.Context, prompt string) (text string, title string, err error)
}

// StreamingSongGenerator — опциональное расширение: вызывает onPartial когда первый клип готов.
type StreamingSongGenerator interface {
	GenerateStreaming(ctx context.Context, lyrics, style string, count int, onPartial func([][]byte)) ([][]byte, error)
}

// AsyncImageGenerator — опциональное расширение: только submit задачи, результат придёт по webhook.
type AsyncImageGenerator interface {
	Submit(ctx context.Context, prompt string, refImages []string, callbackURL string) (taskID string, err error)
}

// AsyncSongGenerator — опциональное расширение: только submit задачи, результат придёт по webhook.
type AsyncSongGenerator interface {
	Submit(ctx context.Context, lyrics, style, callbackURL string) (taskID string, err error)
}

// MockImageGenerator возвращает placeholder PNG.
type MockImageGenerator struct{}

func (g *MockImageGenerator) Generate(ctx context.Context, _ string, _ []string, count int) ([][]byte, error) {
	select {
	case <-time.After(3 * time.Second):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	result := make([][]byte, count)
	for i := range result {
		cp := make([]byte, len(placeholderPNG))
		copy(cp, placeholderPNG)
		result[i] = cp
	}
	return result, nil
}

// MockSongGenerator возвращает placeholder MP3.
type MockSongGenerator struct{}

func (g *MockSongGenerator) Generate(ctx context.Context, _ string, _ string, count int) ([][]byte, error) {
	select {
	case <-time.After(3 * time.Second):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	result := make([][]byte, count)
	for i := range result {
		cp := make([]byte, len(placeholderMP3))
		copy(cp, placeholderMP3)
		result[i] = cp
	}
	return result, nil
}

// NewMockImageReader возвращает placeholder как io.Reader.
func NewMockImageReader() *bytes.Reader {
	return bytes.NewReader(placeholderPNG)
}

// NewMockAudioReader возвращает placeholder как io.Reader.
func NewMockAudioReader() *bytes.Reader {
	return bytes.NewReader(placeholderMP3)
}

var _ = fmt.Sprintf // keep import
