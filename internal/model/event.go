package model

// Event adalah event domain yang bisa diterbitkan ke message broker.
// GetID dipakai sebagai key pesan Kafka, sehingga semua event milik entity
// yang sama masuk ke partisi yang sama dan urutannya terjaga.
type Event interface {
	GetID() string
}
