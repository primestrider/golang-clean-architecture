// Command cli berisi perintah administrasi yang dijalankan manual.
//
// Perintah yang tersedia:
//
//	create-admin --email <email>   jadikan user yang sudah terdaftar sebagai admin
//
// Contoh: go run ./cmd/cli create-admin --email admin@example.com
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/primestrider/golang-clean-architecture/internal/config"
	"github.com/primestrider/golang-clean-architecture/internal/repository"
	"github.com/primestrider/golang-clean-architecture/internal/usecase"
	"github.com/primestrider/golang-clean-architecture/internal/validation"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// run menjalankan subcommand sesuai argumen.
func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: cli create-admin --email <email>")
	}
	switch args[0] {
	case "create-admin":
		return createAdmin(args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

// createAdmin menjadikan user dengan email tertentu sebagai admin. User harus
// sudah mendaftar lebih dulu lewat API.
func createAdmin(args []string) error {
	fs := flag.NewFlagSet("create-admin", flag.ContinueOnError)
	email := fs.String("email", "", "email user yang akan dijadikan admin")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *email == "" {
		return errors.New("--email is required")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := config.NewLogger(cfg)
	db, err := config.NewDatabase(cfg, log)
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	admin := usecase.NewAdminUseCase(log, validation.New(), repository.NewTransactor(db),
		repository.NewUserRepository(db), usecase.NewOutboxPublisher(repository.NewOutboxRepository(db)),
		usecase.NewAuditRecorder(repository.NewAuditRepository(db)))
	user, err := admin.PromoteByEmail(context.Background(), *email)
	if err != nil {
		return err
	}
	fmt.Printf("user %s (%s) is now an admin\n", user.Email, user.ID)
	return nil
}
