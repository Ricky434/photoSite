package models

import (
	"database/sql"
	"errors"
	"time"
)

var (
	ErrDuplicateName      = errors.New("duplicate name")
	ErrRecordNotFound     = errors.New("record not found")
	ErrEditConflict       = errors.New("edit conflict")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidLatLon      = errors.New("Invalid latitude or longitude")

	ImageExtensions = []string{
		".gif",
		//== jpeg
		".jpg",
		".jpeg",
		".jfif",
		".pjpeg",
		".pjp",
		//==
		".png",
		".svg",
		".webp",
	}

	VideoExtensions = []string{
		".mp4",
		".mkv",
	}
)

const (
	ADMIN_LEVEL = 10
)

type Models struct {
	Users  UserModelInterface
	Photos PhotoModelInterface
	Events EventModelInterface
}

func New(db *sql.DB) Models {
	return Models{
		Users:  &UserModel{DB: db},
		Photos: &PhotoModel{DB: db},
		Events: &EventModel{DB: db},
	}
}

// DA SPOSTARE

func newNullInt(n *int) sql.NullInt64 {
	if n == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{
		Int64: int64(*n),
		Valid: true,
	}
}

func newNullFloat(n *float32) sql.NullFloat64 {
	if n == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{
		Float64: float64(*n),
		Valid:   true,
	}
}

func newNullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{
		Time:  *t,
		Valid: true,
	}
}
