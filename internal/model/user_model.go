package model

// UserResponse adalah representasi user yang dikirim ke client. Hash
// password tidak pernah ikut.
type UserResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
	// AvatarURL adalah presigned URL avatar yang berlaku sementara; kosong
	// bila user belum punya avatar atau object storage dimatikan.
	AvatarURL string `json:"avatar_url,omitempty"`
	Version   int    `json:"version"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// RegisterUserRequest adalah input registrasi user baru.
//
// Password dibatasi 8–72 byte: minimal 8 supaya tidak terlalu lemah,
// maksimal 72 karena bcrypt mengabaikan byte setelah ke-72.
type RegisterUserRequest struct {
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=8,max=72"`
	Name     string `json:"name" validate:"required,max=100"`
}

// UpdateUserRequest adalah input pengubahan profil user yang login.
// Field kosong berarti tidak diubah. Mengganti password wajib menyertakan
// password lama (CurrentPassword).
type UpdateUserRequest struct {
	ID              string `json:"-" validate:"required"`
	Name            string `json:"name" validate:"omitempty,max=100"`
	CurrentPassword string `json:"current_password" validate:"omitempty,max=72"`
	NewPassword     string `json:"new_password" validate:"omitempty,min=8,max=72"`
	// Version adalah versi data yang terakhir dilihat client (optimistic locking).
	Version int `json:"version" validate:"required,min=1"`
}

// GetUserRequest mengidentifikasi user yang profilnya diambil.
type GetUserRequest struct {
	ID string `json:"-" validate:"required"`
}
