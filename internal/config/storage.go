package config

import (
	"context"
	"fmt"
	"time"

	"github.com/primestrider/golang-clean-architecture/internal/gateway/storage"
)

// NewStorage membuat client object storage. Mengembalikan nil bila storage
// dimatikan.
func NewStorage(cfg *Config) (*storage.MinioStorage, error) {
	if !cfg.Storage.Enabled {
		return nil, nil
	}
	s, err := storage.New(storage.Options{
		Endpoint:       cfg.Storage.Endpoint,
		PublicEndpoint: cfg.Storage.PublicEndpoint,
		AccessKey:      cfg.Storage.AccessKey,
		SecretKey:      cfg.Storage.SecretKey,
		Bucket:         cfg.Storage.Bucket,
		UseSSL:         cfg.Storage.UseSSL,
	})
	if err != nil {
		return nil, err
	}
	// Server S3 bisa butuh beberapa detik untuk siap setelah container start,
	// jadi pembuatan bucket dicoba ulang sebentar.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for {
		err = s.EnsureBucket(ctx)
		if err == nil {
			return s, nil
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("ensure storage bucket: %w", err)
		case <-time.After(500 * time.Millisecond):
		}
	}
}
