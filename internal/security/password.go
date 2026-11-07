// Package security berisi primitif keamanan: hash password, penerbitan dan
// verifikasi JWT, serta pembuatan refresh token.
//
// Package ini sengaja kecil dan tanpa ketergantungan pada layer lain
// (kecuali apperror), supaya mudah diaudit dan diuji.
package security

import (
	"golang.org/x/crypto/bcrypt"
)

// dummyHash adalah hash bcrypt untuk password acak yang tidak dipakai siapa
// pun. Dibandingkan saat user tidak ditemukan supaya waktu respons login
// sama dengan saat password salah (lihat CompareDummy).
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), bcrypt.DefaultCost)

// HashPassword membuat hash bcrypt dari password. bcrypt sengaja lambat dan
// memakai salt acak, sehingga hash yang bocor sulit ditebak balik.
//
// bcrypt hanya memakai 72 byte pertama password; validasi request membatasi
// panjang password maksimal 72 byte supaya tidak ada bagian yang diabaikan
// diam-diam.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// ComparePassword melaporkan apakah password cocok dengan hash.
func ComparePassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// CompareDummy menjalankan perbandingan bcrypt terhadap hash dummy dan
// membuang hasilnya.
//
// Tanpa ini, login dengan email yang tidak terdaftar akan menjawab jauh
// lebih cepat (tidak ada bcrypt) daripada login dengan password salah.
// Perbedaan waktu itu bisa dipakai penyerang untuk menebak email mana yang
// terdaftar (user enumeration).
func CompareDummy(password string) {
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
}
