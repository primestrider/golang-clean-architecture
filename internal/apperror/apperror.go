// Package apperror mendefinisikan error domain bertipe yang dipakai use case.
//
// Use case tidak boleh tahu tentang HTTP atau gRPC. Karena itu use case
// mengembalikan *Error dengan Code yang netral protokol (NOT_FOUND,
// CONFLICT, ...). Setiap delivery lalu menerjemahkan Code itu ke bahasanya
// sendiri: status HTTP untuk Fiber, codes.Code untuk gRPC.
//
// Field Message selalu aman untuk ditampilkan ke client. Detail teknis
// (error database, stack, dan sejenisnya) disimpan di Err dan hanya masuk ke log.
package apperror

import (
	"errors"
	"fmt"
)

// Code adalah kategori error yang netral terhadap protokol.
type Code string

// Daftar Code yang dikenal. Tambahkan Code baru di sini, lalu petakan di
// delivery HTTP (problem.go) dan gRPC (errors.go).
const (
	CodeBadRequest       Code = "BAD_REQUEST"
	CodeValidation       Code = "VALIDATION_FAILED"
	CodeUnauthorized     Code = "UNAUTHORIZED"
	CodeForbidden        Code = "FORBIDDEN"
	CodeNotFound         Code = "NOT_FOUND"
	CodeConflict         Code = "CONFLICT"
	CodePayloadTooLarge  Code = "PAYLOAD_TOO_LARGE"
	CodeUnsupportedMedia Code = "UNSUPPORTED_MEDIA_TYPE"
	CodeRateLimited      Code = "RATE_LIMITED"
	CodeUnavailable      Code = "UNAVAILABLE"
	CodeInternal         Code = "INTERNAL"
)

// FieldError menjelaskan satu field request yang tidak valid.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error adalah error domain. Message aman dikirim ke client; Err (penyebab
// asli) hanya untuk log dan tidak pernah dikirim ke client.
type Error struct {
	Code    Code
	Message string
	Fields  []FieldError
	Err     error
}

// Error mengimplementasikan interface error. Penyebab ikut ditulis supaya
// log tetap informatif.
func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap memungkinkan errors.Is dan errors.As menembus ke error penyebab.
func (e *Error) Unwrap() error { return e.Err }

// New membuat error dengan code dan pesan untuk client.
func New(code Code, msg string) *Error { return &Error{Code: code, Message: msg} }

// Wrap membuat error dengan code, pesan untuk client, dan penyebab asli.
func Wrap(code Code, msg string, err error) *Error {
	return &Error{Code: code, Message: msg, Err: err}
}

// Validation membuat error validasi berisi daftar field yang salah.
func Validation(fields []FieldError) *Error {
	return &Error{Code: CodeValidation, Message: "request validation failed", Fields: fields}
}

// NotFound membuat error "<resource> not found".
func NotFound(resource string) *Error { return New(CodeNotFound, resource+" not found") }

// Conflict membuat error konflik, misalnya data duplikat atau versi usang.
func Conflict(msg string) *Error { return New(CodeConflict, msg) }

// Unauthorized membuat error untuk request yang belum atau gagal terautentikasi.
func Unauthorized(msg string) *Error { return New(CodeUnauthorized, msg) }

// Forbidden membuat error untuk user yang terautentikasi tetapi tidak berhak.
func Forbidden(msg string) *Error { return New(CodeForbidden, msg) }

// Internal membungkus error tak terduga. Pesannya selalu generik supaya
// detail internal (query, DSN, nama tabel) tidak bocor ke client.
func Internal(err error) *Error {
	return &Error{Code: CodeInternal, Message: "internal server error", Err: err}
}

// As mencari *Error di dalam rantai error.
func As(err error) (*Error, bool) {
	return errors.AsType[*Error](err)
}

// From selalu menghasilkan *Error: mengembalikan *Error yang ada di rantai,
// atau membungkus error lain sebagai Internal.
func From(err error) *Error {
	if e, ok := As(err); ok {
		return e
	}
	return Internal(err)
}
