package repository

import (
	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"gorm.io/gorm"
)

// AuditRepository mengakses tabel audit_logs. Jejak audit hanya ditambah,
// tidak pernah diubah, sehingga cukup memakai Create dari Repository.
type AuditRepository struct {
	Repository[entity.AuditLog]
}

// NewAuditRepository membuat AuditRepository.
func NewAuditRepository(db *gorm.DB) *AuditRepository {
	return &AuditRepository{Repository: Repository[entity.AuditLog]{DB: db, Name: "audit log"}}
}
