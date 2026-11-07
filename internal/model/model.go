package model

// WebResponse adalah amplop standar untuk respons sukses:
// {"data": ..., "paging": ...}. Respons error memakai format Problem
// Details (lihat delivery/http/problem.go), bukan struct ini.
type WebResponse[T any] struct {
	Data   T             `json:"data"`
	Paging *PageMetadata `json:"paging,omitempty"`
}

// PageMetadata menjelaskan posisi halaman pada respons daftar.
type PageMetadata struct {
	Page      int   `json:"page"`
	Size      int   `json:"size"`
	TotalItem int64 `json:"total_item"`
	TotalPage int64 `json:"total_page"`
}

// NewPageMetadata menghitung metadata paging dari total data.
func NewPageMetadata(page, size int, total int64) *PageMetadata {
	totalPage := int64(0)
	if size > 0 {
		totalPage = (total + int64(size) - 1) / int64(size)
	}
	return &PageMetadata{Page: page, Size: size, TotalItem: total, TotalPage: totalPage}
}
