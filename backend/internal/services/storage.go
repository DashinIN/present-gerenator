package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var ErrUnsafeStorageKey = errors.New("unsafe storage key")

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
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("resolve storage dir: %w", err)
	}
	if err := os.MkdirAll(baseAbs, 0755); err != nil {
		return nil, fmt.Errorf("create storage dir: %w", err)
	}
	return &LocalStorage{baseDir: baseAbs, baseURL: strings.TrimRight(baseURL, "/")}, nil
}

func CleanStorageKey(key string) (string, error) {
	if key == "" || strings.ContainsRune(key, '\x00') {
		return "", ErrUnsafeStorageKey
	}

	key = strings.ReplaceAll(key, "\\", "/")
	if strings.HasPrefix(key, "/") {
		return "", ErrUnsafeStorageKey
	}

	for _, part := range strings.Split(key, "/") {
		if part == ".." {
			return "", ErrUnsafeStorageKey
		}
	}

	clean := path.Clean(key)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", ErrUnsafeStorageKey
	}
	return clean, nil
}

func SafeLocalStoragePath(baseDir, key string) (string, error) {
	clean, err := CleanStorageKey(key)
	if err != nil {
		return "", err
	}

	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("resolve base dir: %w", err)
	}
	target := filepath.Join(baseAbs, filepath.FromSlash(clean))
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("resolve target path: %w", err)
	}

	rel, err := filepath.Rel(baseAbs, targetAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return "", ErrUnsafeStorageKey
	}
	return targetAbs, nil
}

func (s *LocalStorage) Upload(_ context.Context, key string, r io.Reader, _ string) error {
	dest, err := SafeLocalStoragePath(s.baseDir, key)
	if err != nil {
		return err
	}
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
	clean, err := CleanStorageKey(key)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/api/files/%s", s.baseURL, clean), nil
}

func (s *LocalStorage) Download(_ context.Context, key string) ([]byte, error) {
	src, err := SafeLocalStoragePath(s.baseDir, key)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(src)
}

func (s *LocalStorage) Delete(_ context.Context, key string) error {
	target, err := SafeLocalStoragePath(s.baseDir, key)
	if err != nil {
		return err
	}
	return os.Remove(target)
}

func StorageKeyURL(baseURL, key string) (string, error) {
	clean, err := CleanStorageKey(key)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/api/files/%s", strings.TrimRight(baseURL, "/"), clean), nil
}
