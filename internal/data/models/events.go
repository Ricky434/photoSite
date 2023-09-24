package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sitoWow/internal/data"
	"time"
)

type EventModelInterface interface {
	Insert(event *Event) error
	Update(event *Event) error
	Delete(id int) error
	GetByName(name string) (*Event, error)
	GetByID(id int) (*Event, error)
	GetAll(filters data.Filters) ([]*Event, error)
}

type EventModel struct {
	DB *sql.DB
}

type Event struct {
	ID      int
	Name    string
	Date    *time.Time
	Version int
}

func (m *EventModel) Insert(event *Event) error {
	query := `
    INSERT INTO events (name, day)
    VALUES ($1, $2)
    RETURNING id, version
    `

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, event.Name, newNullTime(event.Date)).Scan(&event.ID, &event.Version)
	if err != nil {
		if err.Error() == `pq: un valore chiave duplicato viola il vincolo univoco "events_name_key"` ||
			err.Error() == `pq: duplicate key value violates unique constraint "events_name_key"` {
			return ErrDuplicateName
		}

		return err
	}

	return nil
}

func (m *EventModel) Update(event *Event) error {
	query := `
    UPDATE events
    SET name = $1, day = $2, version = version + 1
    WHERE id = $3 AND version = $4
    RETURNING version
    `

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, event.Name, newNullTime(event.Date), event.ID, event.Version).Scan(&event.Version)
	if err != nil {
		switch {
		case err.Error() == `pq: un valore chiave duplicato viola il vincolo univoco "events_name_key"` ||
			err.Error() == `pq: duplicate key value violates unique constraint "events_name_key"`:
			return ErrDuplicateName
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}

	return nil
}

func (m *EventModel) Delete(id int) error {
	query := `
    DELETE FROM events
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

// TODO: probabilmente conviene unire questo e byID in getall
func (m *EventModel) GetByName(name string) (*Event, error) {
	query := `
    SELECT id, name, day, version
    FROM events
    WHERE name = $1
    `

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var event Event

	err := m.DB.QueryRowContext(ctx, query, name).Scan(&event.ID, &event.Name, &event.Date, &event.Version)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRecordNotFound
		}

		return nil, err
	}

	return &event, nil
}

func (m *EventModel) GetByID(id int) (*Event, error) {
	query := `
    SELECT id, name, day, version
    FROM events
    WHERE id = $1
    `

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var event Event

	err := m.DB.QueryRowContext(ctx, query, id).Scan(&event.ID, &event.Name, &event.Date, &event.Version)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRecordNotFound
		}

		return nil, err
	}

	return &event, nil
}

func (m *EventModel) GetAll(filters data.Filters) ([]*Event, error) {
	query := fmt.Sprintf(`
    SELECT id, name, day, version
    FROM events
    ORDER BY %s %s, name ASC
    LIMIT $1 OFFSET $2
    `, filters.SortColumn(), filters.SortDirection())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, filters.Limit(), filters.Offset())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*Event

	for rows.Next() {
		var event Event

		rows.Scan(
			&event.ID,
			&event.Name,
			&event.Date,
			&event.Version,
		)
		if err != nil {
			return nil, err
		}

		events = append(events, &event)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return events, nil
}
