// Package usecase berisi logika bisnis aplikasi.
//
// Use case tidak tahu apa-apa tentang HTTP, gRPC, maupun Kafka. Ia menerima
// model request, menjalankan aturan bisnis (validasi, transaksi, otorisasi
// kepemilikan data), lalu mengembalikan model respons atau error domain.
package usecase
