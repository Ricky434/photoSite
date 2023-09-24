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
	Delete(id int) error
	GetAll(event *int32, filters data.Filters) ([]*Photo, data.Metadata, error)
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
	Event     *int32
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

func (m *PhotoModel) GetAll(event *int32, filters data.Filters) ([]*Photo, data.Metadata, error) {
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
	query1 := `
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

	rows, err := m.DB.QueryContext(ctx, query1, n)
	if err != nil {
		return nil, fmt.Errorf("query1: %w", err)
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
			return nil, fmt.Errorf("query1: %w", err)
		}

		photos = append(photos, &photo)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("query1: %w", err)
	}

	// Get first n without event
	query2 := `
    SELECT id, file_name, created_at, taken_at, latitude, longitude, event
    FROM photos
    WHERE event IS NULL
    ORDER BY taken_at ASC
    LIMIT $1
    `
	ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel2()

	rows2, err := m.DB.QueryContext(ctx2, query2, n)
	if err != nil {
		return nil, fmt.Errorf("query2: %w", err)
	}

	defer rows2.Close()

	for rows2.Next() {
		var photo Photo

		err := rows2.Scan(
			&photo.ID,
			&photo.FileName,
			&photo.CreatedAt,
			&photo.TakenAt,
			&photo.Latitude,
			&photo.Longitude,
			&photo.Event,
		)
		if err != nil {
			return nil, fmt.Errorf("query2: %w", err)
		}

		photos = append(photos, &photo)
	}

	if err = rows2.Err(); err != nil {
		return nil, fmt.Errorf("query2: %w", err)
	}

	return photos, nil
}
