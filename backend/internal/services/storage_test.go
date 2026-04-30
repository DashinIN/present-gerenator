package services

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeLocalStoragePathRejectsUnsafeKeys(t *testing.T) {
	base := t.TempDir()

	unsafeKeys := []string{
		"",
		"../secret.txt",
		"..\\secret.txt",
		"uploads/../../secret.txt",
		"/absolute/path.txt",
		string([]byte{'o', 'k', 0, 'x'}),
	}

	for _, key := range unsafeKeys {
		t.Run(key, func(t *testing.T) {
			if _, err := SafeLocalStoragePath(base, key); !errors.Is(err, ErrUnsafeStorageKey) {
				t.Fatalf("expected ErrUnsafeStorageKey, got %v", err)
			}
		})
	}
}

func TestSafeLocalStoragePathAllowsKeysInsideBaseDir(t *testing.T) {
	base := t.TempDir()

	got, err := SafeLocalStoragePath(base, "uploads/42/file.png")
	if err != nil {
		t.Fatalf("expected valid path, got %v", err)
	}

	baseAbs, err := filepath.Abs(base)
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(baseAbs, got)
	if err != nil {
		t.Fatal(err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		t.Fatalf("resolved path escaped base: %s", got)
	}
}

func TestLocalStorageUploadRejectsUnsafeKey(t *testing.T) {
	storage, err := NewLocalStorage(t.TempDir(), "http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}

	err = storage.Upload(t.Context(), "../secret.txt", bytes.NewBufferString("data"), "text/plain")
	if !errors.Is(err, ErrUnsafeStorageKey) {
		t.Fatalf("expected ErrUnsafeStorageKey, got %v", err)
	}
}
