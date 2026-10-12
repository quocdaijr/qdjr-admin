package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LocalStorage writes objects under a directory. Development only: the API
// serves the directory at /media-files/ (config refuses it in production).
type LocalStorage struct {
	dir string
}

// NewLocalStorage returns a LocalStorage rooted at dir.
func NewLocalStorage(dir string) *LocalStorage {
	return &LocalStorage{dir: dir}
}

// Dir is the root directory (served by the router).
func (l *LocalStorage) Dir() string { return l.dir }

func (l *LocalStorage) path(key string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(key))
	if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("invalid storage key %q", key)
	}
	return filepath.Join(l.dir, clean), nil
}

// Put writes body to dir/key.
func (l *LocalStorage) Put(_ context.Context, key, _ string, body io.Reader, _ int64) error {
	p, err := l.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("local storage mkdir: %w", err)
	}
	f, err := os.Create(p)
	if err != nil {
		return fmt.Errorf("local storage create: %w", err)
	}
	if _, err := io.Copy(f, body); err != nil {
		_ = f.Close()
		return fmt.Errorf("local storage write: %w", err)
	}
	return f.Close()
}

// Delete removes dir/key; a missing file is not an error.
func (l *LocalStorage) Delete(_ context.Context, key string) error {
	p, err := l.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("local storage delete: %w", err)
	}
	return nil
}
