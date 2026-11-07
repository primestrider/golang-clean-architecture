package model

// UserResponse adalah representasi user yang dikirim ke client.
type UserResponse struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Token     string `json:"token,omitempty"`
	CreatedAt int64  `json:"created_at,omitempty"`
	UpdatedAt int64  `json:"updated_at,omitempty"`
}

// VerifyUserRequest berisi token yang akan diverifikasi middleware.
type VerifyUserRequest struct {
	Token string `validate:"required,max=100"`
}

// RegisterUserRequest adalah input registrasi user baru.
type RegisterUserRequest struct {
	ID       string `json:"id" validate:"required,max=100"`
	Password string `json:"password" validate:"required,max=100"`
	Name     string `json:"name" validate:"required,max=100"`
}

// UpdateUserRequest adalah input pengubahan profil. Field kosong berarti
// tidak diubah.
type UpdateUserRequest struct {
	ID       string `json:"-" validate:"required,max=100"`
	Password string `json:"password,omitempty" validate:"max=100"`
	Name     string `json:"name,omitempty" validate:"max=100"`
}

// LoginUserRequest adalah kredensial login.
type LoginUserRequest struct {
	ID       string `json:"id" validate:"required,max=100"`
	Password string `json:"password" validate:"required,max=100"`
}

// LogoutUserRequest mengidentifikasi user yang logout.
type LogoutUserRequest struct {
	ID string `json:"id" validate:"required,max=100"`
}

// GetUserRequest mengidentifikasi user yang profilnya diambil.
type GetUserRequest struct {
	ID string `json:"id" validate:"required,max=100"`
}
