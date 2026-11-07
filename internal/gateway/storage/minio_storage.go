// Package storage berisi implementasi port usecase.ObjectStorage untuk
// storage S3-compatible (MinIO, AWS S3, Cloudflare R2) lewat minio-go.
package storage

import (
	"context"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// region dipakai untuk menandatangani URL tanpa harus menanyakan region
// bucket ke server (presign untuk endpoint publik bisa dihitung offline).
const region = "us-east-1"

// MinioStorage menyimpan object di bucket S3-compatible.
type MinioStorage struct {
	client *minio.Client
	// presigner menandatangani URL memakai endpoint yang bisa diakses client
	// (misalnya localhost:9000), yang bisa berbeda dari endpoint internal
	// yang dipakai aplikasi (misalnya minio:9000 di jaringan Docker).
	presigner *minio.Client
	bucket    string
}

// Options berisi pengaturan koneksi storage.
type Options struct {
	Endpoint       string
	PublicEndpoint string
	AccessKey      string
	SecretKey      string
	Bucket         string
	UseSSL         bool
}

// New membuat MinioStorage.
func New(opts Options) (*MinioStorage, error) {
	creds := credentials.NewStaticV4(opts.AccessKey, opts.SecretKey, "")
	client, err := minio.New(opts.Endpoint, &minio.Options{Creds: creds, Secure: opts.UseSSL, Region: region})
	if err != nil {
		return nil, err
	}
	presigner := client
	if opts.PublicEndpoint != "" && opts.PublicEndpoint != opts.Endpoint {
		if presigner, err = minio.New(opts.PublicEndpoint, &minio.Options{Creds: creds, Secure: opts.UseSSL, Region: region}); err != nil {
			return nil, err
		}
	}
	return &MinioStorage{client: client, presigner: presigner, bucket: opts.Bucket}, nil
}

// Put mengunggah object dengan tipe konten tertentu.
func (s *MinioStorage) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, r, size, minio.PutObjectOptions{ContentType: contentType})
	return err
}

// Delete menghapus object.
func (s *MinioStorage) Delete(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

// PresignGet membuat URL unduhan sementara untuk object privat.
func (s *MinioStorage) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	u, err := s.presigner.PresignedGetObject(ctx, s.bucket, key, ttl, url.Values{})
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// EnsureBucket membuat bucket bila belum ada. Dipanggil saat startup supaya
// lingkungan development langsung siap; di production, bucket biasanya
// sudah disiapkan dan pemanggilan ini hanya memastikan bucket ada.
func (s *MinioStorage) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{Region: region})
}

// Ping memastikan bucket bisa diakses (untuk readiness check).
func (s *MinioStorage) Ping(ctx context.Context) error {
	_, err := s.client.BucketExists(ctx, s.bucket)
	return err
}
