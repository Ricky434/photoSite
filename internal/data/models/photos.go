package models

import (
	"context"
	"database/sql"
	"fmt"
	"sitoWow/internal/data"
	"time"
)

type PhotoModelInterface interface {
	Insert(photo *Photo) error
	Delete(id string) error
	GetAll(event *int32, filters data.Filters) ([]*Photo, data.Metadata, error)
}

type PhotoModel struct {
	DB *sql.DB
}

type Photo struct {
	ID        string
	CreatedAt time.Time
	TakenAt   *time.Time
	Latitude  *float32
	Longitude *float32
	Event     *int32
}

func (m *PhotoModel) Insert(photo *Photo) error {
	query := `
    INSERT INTO photos (taken_at, latitude, longitude, event)
    VALUES ($1, $2, $3, $4)
    RETURNING id, created_at
    `

	args := []any{
		newNullTime(photo.TakenAt),
		newNullFloat(photo.Latitude),
		newNullFloat(photo.Longitude),
		newNullInt(photo.Event),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&photo.ID, &photo.CreatedAt)
	if err != nil {
		return err
	}

	return nil
}

func (m *PhotoModel) Delete(id string) error {
	query := `
    DELETE FROM photos
    WHERE id = $1
    `

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := m.DB.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	count, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrRecordNotFound
	}

	return nil
}

func (m *PhotoModel) GetAll(event *int32, filters data.Filters) ([]*Photo, data.Metadata, error) {
	query := fmt.Sprintf(`
    SELECT COUNT(*), id, created_at, taken_at, latitude, longitude, event
    FROM photos
    WHERE event = $1 OR event = NULL
    ORDER BY %s %s, taken_at ASC
    LIMIT $2 OFFSET $4
    `, filters.SortColumn(), filters.SortDirection())

	args := []any{newNullInt(event), filters.Limit(), filters.Offset()}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, data.Metadata{}, err
	}

	defer rows.Close()

	photos := []*Photo{}
	totalRecords := 0

	for rows.Next() {
		var photo *Photo

		err := rows.Scan(
			&totalRecords,
			&photo.ID,
			&photo.CreatedAt,
			&photo.TakenAt,
			&photo.Latitude,
			&photo.Longitude,
			&photo.Event,
		)
		if err != nil {
			return nil, data.Metadata{}, err
		}

		photos = append(photos, photo)
	}

	if err = rows.Err(); err != nil {
		return nil, data.Metadata{}, err
	}

	metadata := data.CalculateMetadata(totalRecords, filters.Page, filters.PageSize)
	return photos, metadata, nil
}
