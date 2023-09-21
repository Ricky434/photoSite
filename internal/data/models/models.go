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

func newNullInt(n *int32) sql.NullInt32 {
	if n == nil {
		return sql.NullInt32{}
	}
	return sql.NullInt32{
		Int32: *n,
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
