// Package config adalah composition root aplikasi: tempat semua dependency
// (database, logger, Fiber, Kafka, use case, controller) dibuat dan dirangkai
// secara manual dan eksplisit.
//
// Tidak ada DI container di sini dengan sengaja. Membaca file di package ini
// dari atas ke bawah seharusnya cukup untuk memahami bagaimana seluruh
// aplikasi tersusun.
package config
