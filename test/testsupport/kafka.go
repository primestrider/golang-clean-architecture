package testsupport

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/kafka"
)

// kafkaImage adalah image Kafka (mode KRaft, tanpa Zookeeper) untuk test.
const kafkaImage = "confluentinc/confluent-local:7.5.0"

// StartKafka menjalankan container Kafka untuk satu test dan mengembalikan
// alamat broker-nya. Container dimatikan otomatis saat test selesai.
// Topik dibuat otomatis saat pertama kali dipakai.
func StartKafka(t testing.TB) []string {
	t.Helper()
	ctx := context.Background()
	container, err := kafka.Run(ctx, kafkaImage, kafka.WithClusterID("gca-test"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	brokers, err := container.Brokers(ctx)
	require.NoError(t, err)
	return brokers
}
