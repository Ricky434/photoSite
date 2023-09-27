package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sitoWow/internal/data"
	"time"
)

type PhotoModelInterface interface {
	Insert(photo *Photo) error
	Delete(id int) error
	GetByFile(file string) (*Photo, error)
	GetAll(event *int, filters data.Filters) ([]*Photo, data.Metadata, error)
	Summary(n int) ([]*Photo, error)
}

type PhotoModel struct {
	DB *sql.DB
}

type Photo struct {
	ID        int
	FileName  string
	CreatedAt time.Time
	TakenAt   *time.Time
	Latitude  *float32
	Longitude *float32
	Event     int
}

func (m *PhotoModel) Insert(photo *Photo) error {
	query := `
    INSERT INTO photos (file_name, taken_at, latitude, longitude, event)
    VALUES ($1, $2, $3, $4, $5)
    RETURNING id, created_at
    `

	args := []any{
		photo.FileName,
		newNullTime(photo.TakenAt),
		newNullFloat(photo.Latitude),
		newNullFloat(photo.Longitude),
		photo.Event,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&photo.ID, &photo.CreatedAt)
	if err != nil {
		return err
	}

	return nil
}

func (m *PhotoModel) Delete(id int) error {
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

func (m *PhotoModel) GetByFile(file string) (*Photo, error) {
	query := `
    SELECT id, file_name, created_at, taken_at, latitude, longitude, event
    FROM photos
    WHERE file_name = $1
    `

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var photo Photo
	err := m.DB.QueryRowContext(ctx, query, file).Scan(
		&photo.ID,
		&photo.FileName,
		&photo.CreatedAt,
		&photo.TakenAt,
		&photo.Latitude,
		&photo.Longitude,
		&photo.Event,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	return &photo, nil
}

func (m *PhotoModel) GetAll(event *int, filters data.Filters) ([]*Photo, data.Metadata, error) {
	query := fmt.Sprintf(`
    SELECT COUNT(*) OVER(), photos.id, file_name, created_at, taken_at, latitude, longitude, event
    FROM photos LEFT JOIN events ON event = events.id
    WHERE event = $1 OR $1 IS NULL
    ORDER BY %s %s, taken_at ASC
    LIMIT $2 OFFSET $3
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
		var photo Photo

		err := rows.Scan(
			&totalRecords,
			&photo.ID,
			&photo.FileName,
			&photo.CreatedAt,
			&photo.TakenAt,
			&photo.Latitude,
			&photo.Longitude,
			&photo.Event,
		)
		if err != nil {
			return nil, data.Metadata{}, err
		}

		photos = append(photos, &photo)
	}

	if err = rows.Err(); err != nil {
		return nil, data.Metadata{}, err
	}

	metadata := data.CalculateMetadata(totalRecords, filters.Page, filters.PageSize)
	return photos, metadata, nil
}

// Returns the first n photos for each event, both ordered by date
func (m *PhotoModel) Summary(n int) ([]*Photo, error) {
	query := `
    SELECT l.id, l.file_name, l.created_at, l.taken_at, l.latitude, l.longitude, l.event
    FROM events AS e, lateral (
        SELECT * 
        FROM photos
        WHERE photos.event = e.id 
        ORDER BY taken_at
        LIMIT $1
    ) as l
    ORDER BY e.day ASC, taken_at ASC
    `
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, n)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	photos := []*Photo{}

	for rows.Next() {
		var photo Photo

		err := rows.Scan(
			&photo.ID,
			&photo.FileName,
			&photo.CreatedAt,
			&photo.TakenAt,
			&photo.Latitude,
			&photo.Longitude,
			&photo.Event,
		)
		if err != nil {
			return nil, err
		}

		photos = append(photos, &photo)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return photos, nil
}
