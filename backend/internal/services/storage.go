package services

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type StorageService interface {
	Upload(ctx context.Context, key string, r io.Reader, contentType string) error
	Download(ctx context.Context, key string) ([]byte, error)
	GetURL(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
}

// LocalStorage хранит файлы на диске — мок для разработки.
type LocalStorage struct {
	baseDir string
	baseURL string
}

func NewLocalStorage(baseDir, baseURL string) (*LocalStorage, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("create storage dir: %w", err)
	}
	return &LocalStorage{baseDir: baseDir, baseURL: strings.TrimRight(baseURL, "/")}, nil
}

func (s *LocalStorage) Upload(_ context.Context, key string, r io.Reader, _ string) error {
	dest := filepath.Join(s.baseDir, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func (s *LocalStorage) GetURL(_ context.Context, key string) (string, error) {
	return fmt.Sprintf("%s/api/files/%s", s.baseURL, key), nil
}

func (s *LocalStorage) Download(_ context.Context, key string) ([]byte, error) {
	return os.ReadFile(filepath.Join(s.baseDir, filepath.FromSlash(key)))
}

func (s *LocalStorage) Delete(_ context.Context, key string) error {
	return os.Remove(filepath.Join(s.baseDir, filepath.FromSlash(key)))
}
