// Package http adalah delivery layer untuk protokol HTTP (Fiber).
//
// Controller di sini hanya bertugas menerjemahkan request HTTP menjadi model
// request, memanggil use case, lalu menerjemahkan hasilnya kembali menjadi
// respons HTTP. Logika bisnis tidak boleh ada di sini.
package http
