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
	DeleteByFile(file string) error
	GetByFile(file string) (*Photo, error)
	GetFiltered(event *int, filters data.Filters) ([]*Photo, data.Metadata, error) // Not used
	GetAll(event *int) ([]*Photo, error)
	Summary(n int) ([]*Photo, error)
}

type PhotoModel struct {
	DB *sql.DB
}

type Photo struct {
	ID           int
	FileName     string
	ThumbName    string
	CreatedAt    time.Time
	TakenAt      *time.Time
	Latitude     *float32
	Longitude    *float32
	Event        int
	PreviousFile *string
	NextFile     *string
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
		//TODO errore versione italiana
		if err.Error() == `pq: new row for relation "photos" violates check constraint "valid_coords"` {
			return ErrInvalidLatLon
		}
		if err.Error() == `pq: un valore chiave duplicato viola il vincolo univoco "photos_file_name_key"` ||
			err.Error() == `pq: duplicate key value violates unique constraint "photos_file_name_key"` {
			return ErrDuplicateName
		}
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

func (m *PhotoModel) DeleteByFile(file string) error {
	query := `
    DELETE FROM photos
    WHERE file_name = $1
    `

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := m.DB.ExecContext(ctx, query, file)
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

// Get photo by file, along with its previous and next photos' file names (in the same event)
func (m *PhotoModel) GetByFile(file string) (*Photo, error) {
	query := `
	SELECT *
	FROM (
		SELECT id, file_name, created_at, taken_at, latitude, longitude, event,
				lag(file_name) over (order by taken_at asc, id asc) as prev,
				lead(file_name) over (order by taken_at asc, id asc) as next
		FROM photos
		WHERE event = (select event from photos where file_name = $1)
	) x
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
		&photo.PreviousFile,
		&photo.NextFile,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	return &photo, nil
}

func (m *PhotoModel) GetAll(event *int) ([]*Photo, error) {
	query := `
    SELECT photos.id, file_name, created_at, taken_at, latitude, longitude, event
    FROM photos LEFT JOIN events ON event = events.id
    WHERE event = $1 OR $1 IS NULL
    ORDER BY taken_at ASC, photos.id`
	// IMPORTANT: the order by photos.id is necessary because in case of ties in ordering
	// posgres makes no guarantee about what the ordering will be, so records could be
	// seen as "moving around". Thus, we need an attribute that cannot be tied

	args := []any{newNullInt(event)}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, args...)
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

func (m *PhotoModel) GetFiltered(event *int, filters data.Filters) ([]*Photo, data.Metadata, error) {
	query := fmt.Sprintf(`
    SELECT COUNT(*) OVER(), photos.id, file_name, created_at, taken_at, latitude, longitude, event
    FROM photos LEFT JOIN events ON event = events.id
    WHERE event = $1 OR $1 IS NULL
    ORDER BY %s %s, taken_at ASC, photos.id
    LIMIT $2 OFFSET $3
    `, filters.SortColumn(), filters.SortDirection())
	// IMPORTANT: the order by photos.id is necessary because in case of ties in ordering
	// posgres makes no guarantee about what the ordering will be, so records could be
	// seen as "moving around". Thus, we need an attribute that cannot be tied

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
        ORDER BY taken_at ASC, photos.id ASC
        LIMIT $1
    ) as l
    ORDER BY e.day ASC, taken_at ASC, l.id ASC
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
