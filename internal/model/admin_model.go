package model

// ListUsersRequest adalah paging untuk daftar user (khusus admin).
type ListUsersRequest struct {
	Page int `json:"page" validate:"min=1"`
	Size int `json:"size" validate:"min=1,max=100"`
}

// ChangeRoleRequest mengubah role seorang user. ActorID adalah admin yang
// melakukan perubahan, diisi dari token login.
type ChangeRoleRequest struct {
	ActorID string `json:"-" validate:"required"`
	UserID  string `json:"-" validate:"required"`
	Role    string `json:"role" validate:"required,oneof=user admin"`
	// Version adalah versi data yang terakhir dilihat client (optimistic locking).
	Version int `json:"version" validate:"required,min=1"`
}
