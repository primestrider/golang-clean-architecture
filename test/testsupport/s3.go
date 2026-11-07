package testsupport

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	miniogo "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// s3Image adalah server S3-compatible untuk test (SeaweedFS). Aplikasi
// memakai client S3 generik (minio-go), jadi server S3-compatible apa pun
// bisa dipakai: AWS S3, MinIO, Cloudflare R2, atau SeaweedFS.
const s3Image = "chrislusf/seaweedfs:4.50"

// StartS3 menjalankan server S3-compatible dan mengembalikan endpoint
// (host:port), access key, dan secret key. Kredensial diambil dari
// deploy/seaweedfs-s3.json. Bucket dibuat oleh aplikasi saat startup
// (EnsureBucket).
func StartS3(t testing.TB) (endpoint, accessKey, secretKey string) {
	t.Helper()
	ctx := context.Background()
	container, err := testcontainers.Run(ctx, s3Image,
		testcontainers.WithCmd("server", "-s3", "-s3.config=/etc/seaweedfs/s3.json", "-dir=/data",
			"-ip.bind=0.0.0.0", "-master.volumeSizeLimitMB=64"),
		testcontainers.WithFiles(testcontainers.ContainerFile{
			HostFilePath:      filepath.Join(RepoRoot(), "deploy", "seaweedfs-s3.json"),
			ContainerFilePath: "/etc/seaweedfs/s3.json",
			FileMode:          0o644,
		}),
		testcontainers.WithExposedPorts("8333/tcp"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("8333/tcp")),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	endpoint, err = container.PortEndpoint(ctx, "8333/tcp", "")
	require.NoError(t, err)
	warmUpS3(t, endpoint, "devaccesskey", "devsecretkey")
	return endpoint, "devaccesskey", "devsecretkey"
}

// warmUpS3 melakukan satu penulisan awal. Penulisan pertama di SeaweedFS
// memicu alokasi volume yang bisa memakan waktu lebih lama dari timeout
// request aplikasi, jadi dilakukan sekali di sini sebelum test berjalan.
func warmUpS3(t testing.TB, endpoint, accessKey, secretKey string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	client, err := miniogo.New(endpoint, &miniogo.Options{Creds: credentials.NewStaticV4(accessKey, secretKey, ""), Region: "us-east-1"})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return client.MakeBucket(ctx, "warmup", miniogo.MakeBucketOptions{Region: "us-east-1"}) == nil
	}, time.Minute, time.Second)
	_, err = client.PutObject(ctx, "warmup", "warmup.txt", bytes.NewReader([]byte("ok")), 2, miniogo.PutObjectOptions{})
	require.NoError(t, err)
}
