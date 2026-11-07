package entity

// RefreshToken adalah satu refresh token yang pernah diterbitkan.
//
// Yang disimpan hanya hash token (TokenHash), bukan token aslinya. Token
// hasil rotasi dari satu login berbagi FamilyID; ReplacedByID menunjuk ke
// token pengganti. Token yang sudah dicabut (RevokedAt terisi) tetapi dipakai
// lagi adalah tanda pencurian token.
type RefreshToken struct {
	ID           string  `gorm:"column:id;primaryKey"`
	UserID       string  `gorm:"column:user_id"`
	FamilyID     string  `gorm:"column:family_id"`
	TokenHash    string  `gorm:"column:token_hash"`
	ExpiresAt    int64   `gorm:"column:expires_at"`
	RevokedAt    *int64  `gorm:"column:revoked_at"`
	ReplacedByID *string `gorm:"column:replaced_by_id"`
	UserAgent    string  `gorm:"column:user_agent"`
	IP           string  `gorm:"column:ip"`
	CreatedAt    int64   `gorm:"column:created_at;autoCreateTime:milli"`
}

// TableName memberi tahu GORM nama tabel untuk entity ini.
func (r *RefreshToken) TableName() string {
	return "refresh_tokens"
}
