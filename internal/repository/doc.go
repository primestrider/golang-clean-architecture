// Package repository adalah layer akses data. Hanya package ini yang menulis
// query database (GORM).
//
// Use case memanggil repository lewat interface, sehingga sumber data bisa
// diganti atau di-mock di unit test.
package repository
