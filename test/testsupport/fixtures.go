package testsupport

import (
	"testing"

	"github.com/google/uuid"
	"github.com/primestrider/golang-clean-architecture/internal/config"
	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// CreateUser menyimpan user (role "user") langsung ke database tanpa lewat
// API dan mengembalikannya. Gunakan untuk menyiapkan data; alur registrasi
// diuji lewat API di test registrasi itu sendiri.
func (a *TestApp) CreateUser(t testing.TB, email, password string) *entity.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	user := &entity.User{
		ID: uuid.NewString(), Email: email, Name: "User " + email,
		Password: string(hash), Role: entity.RoleUser, Version: 1,
	}
	require.NoError(t, a.DB.Create(user).Error)
	return user
}

// TokenFor menerbitkan access token untuk user tanpa lewat endpoint login.
// Berguna untuk test yang tidak sedang menguji login itu sendiri.
func (a *TestApp) TokenFor(t testing.TB, user *entity.User) string {
	t.Helper()
	token, _, err := config.NewTokenIssuer(a.Cfg).Issue(user.ID, user.Role)
	require.NoError(t, err)
	return token
}

// CreateContact menyimpan kontak milik userID langsung ke database.
func (a *TestApp) CreateContact(t testing.TB, userID, firstName string) *entity.Contact {
	t.Helper()
	contact := &entity.Contact{
		ID: uuid.NewString(), UserID: userID, FirstName: firstName,
		Email: firstName + "@example.com", Phone: "0800000000",
	}
	require.NoError(t, a.DB.Create(contact).Error)
	return contact
}

// CreateAddress menyimpan alamat milik contactID langsung ke database.
func (a *TestApp) CreateAddress(t testing.TB, contactID, city string) *entity.Address {
	t.Helper()
	address := &entity.Address{
		ID: uuid.NewString(), ContactID: contactID, Street: "Jalan Belum Jadi",
		City: city, Province: "DKI Jakarta", PostalCode: "12345", Country: "Indonesia",
	}
	require.NoError(t, a.DB.Create(address).Error)
	return address
}
