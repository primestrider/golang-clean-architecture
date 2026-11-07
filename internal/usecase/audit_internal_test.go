package usecase

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type auditSample struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	TokenHash string `json:"token_hash"`
	UpdatedAt int64  `json:"updated_at"`
}

func TestDiffChanges_OnlyChangedFields(t *testing.T) {
	before := auditSample{Name: "Budi", Email: "budi@example.com", UpdatedAt: 1}
	after := auditSample{Name: "Budi Baru", Email: "budi@example.com", UpdatedAt: 2}

	changes := diffChanges(before, after)

	assert.Equal(t, map[string][2]any{"name": {"Budi", "Budi Baru"}}, changes)
}

func TestDiffChanges_ExcludesSensitiveFields(t *testing.T) {
	before := auditSample{Password: "old-hash", TokenHash: "a"}
	after := auditSample{Password: "new-hash", TokenHash: "b"}

	changes := diffChanges(before, after)

	assert.Empty(t, changes)
}

func TestDiffChanges_CreateHasOnlyAfterValues(t *testing.T) {
	changes := diffChanges(nil, auditSample{Name: "Budi"})

	assert.Equal(t, [2]any{nil, "Budi"}, changes["name"])
	assert.NotContains(t, changes, "password")
}

func TestDiffChanges_ExtraExclusions(t *testing.T) {
	changes := diffChanges(auditSample{Name: "a", Email: "x"}, auditSample{Name: "b", Email: "y"}, "email")

	assert.Equal(t, map[string][2]any{"name": {"a", "b"}}, changes)
}
