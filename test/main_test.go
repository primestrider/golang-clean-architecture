//go:build integration

// Package test berisi integration test end-to-end: request HTTP sungguhan
// ke aplikasi lengkap yang terhubung ke MySQL di container Docker.
package test

import (
	"os"
	"testing"

	"github.com/primestrider/golang-clean-architecture/test/testsupport"
)

// TestMain menyalakan satu container MySQL untuk seluruh test di package
// ini. Setiap test lalu memanggil testsupport.NewWebApp yang mengosongkan
// tabel, sehingga test tetap independen walau berbagi container.
func TestMain(m *testing.M) {
	os.Exit(testsupport.RunMain(m))
}
