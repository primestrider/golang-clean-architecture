package testsupport

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// CreateUser menyimpan user langsung ke database (tanpa lewat API) dan
// mengembalikannya. Gunakan untuk menyiapkan data; uji alur registrasi
// lewat API di test registrasi itu sendiri.
func (a *TestApp) CreateUser(t testing.TB, id, password string) *entity.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	user := &entity.User{ID: id, Name: "User " + id, Password: string(hash)}
	require.NoError(t, a.DB.Create(user).Error)
	return user
}

// Login masuk lewat API dan mengembalikan token untuk header Authorization.
func (a *TestApp) Login(t testing.TB, id, password string) string {
	t.Helper()
	resp, raw := a.Do(t, http.MethodPost, "/api/v1/users/_login", model.LoginUserRequest{ID: id, Password: password}, "")
	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))
	return DecodeData[model.UserResponse](t, raw).Token
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
