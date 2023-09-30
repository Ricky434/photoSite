package models

import (
	"context"
	"database/sql"
	"errors"
	"sitoWow/internal/validator"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type UserModelInterface interface {
	Insert(user *User) error
	Authenticate(name, password string) (int, error)
	GetById(id int) (*User, error)
	Exists(id int) (bool, int, error)
	Update(user *User) error
}

type UserModel struct {
	DB *sql.DB
}

type User struct {
	ID        int
	Name      string
	Password  password
	Level     int
	CreatedAt time.Time
	Version   int
}

type password struct {
	plaintext *string
	hash      []byte
}

func (p *password) Set(plaintextPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintextPassword), 12)
	if err != nil {
		return err
	}

	p.plaintext = &plaintextPassword
	p.hash = hash

	return nil
}

func ValidateUser(v *validator.Validator, user *User) {
	v.CheckField(validator.NotBlank(user.Name), "name", "This field cannot be blank")
	v.CheckField(validator.CharsCount(user.Name, 0, 500), "name", "Username must be at most 500 characters long")
	v.CheckField(user.Level >= 0 && user.Level <= 10, "level", "Level must be between 0 and 10")

	if user.Password.plaintext != nil {
		v.CheckField(validator.NotBlank(*user.Password.plaintext), "password", "This field must not be blank")
		v.CheckField(validator.CharsCount(*user.Password.plaintext, 8, 72), "password", "Password must be between 8 and 72 characters long")
	}

	// Logic error, so a panic is more correct
	if user.Password.hash == nil {
		panic("missing password hash for user")
	}
}

func (m *UserModel) Insert(user *User) error {
	query := `
    INSERT INTO users (name, password_hash, level)
    VALUES ($1, $2, $3)
    RETURNING id, created_at, level, version
    `

	args := []any{user.Name, user.Password.hash, user.Level}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&user.ID, &user.CreatedAt, &user.Level, &user.Version)
	if err != nil {
		if err.Error() == `pq: un valore chiave duplicato viola il vincolo univoco "users_name_key"` ||
			err.Error() == `pq: duplicate key value violates unique constraint "users_name_key"` {
			return ErrDuplicateName
		}

		return err
	}

	return nil
}

func (m *UserModel) Authenticate(name, password string) (int, error) {
	var id int
	var hashedPassword []byte

	query := `
    SELECT id, password_hash FROM users WHERE name = $1
    `

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, name).Scan(&id, &hashedPassword)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrInvalidCredentials
		} else {
			return 0, err
		}
	}

	err = bcrypt.CompareHashAndPassword(hashedPassword, []byte(password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return 0, ErrInvalidCredentials
		} else {
			return 0, err
		}
	}

	return id, nil
}

func (m *UserModel) GetById(id int) (*User, error) {
	query := `
    SELECT id, created_at, name, password_hash, level, version
    FROM users
    WHERE id = $1
    `

	var user User

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.Name,
		&user.Password.hash,
		&user.Level,
		&user.Version,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRecordNotFound
		}

		return nil, err
	}

	return &user, nil
}

func (m *UserModel) Exists(id int) (bool, int, error) {
	user, err := m.GetById(id)
	if err != nil {
		if errors.Is(err, ErrRecordNotFound) {
			return false, 0, nil
		}

		return false, 0, err
	}

	return true, user.Level, nil
}

func (m *UserModel) Update(user *User) error {
	query := `
    UPDATE users
    SET name = $1, password_hash = $2, level = $3, version = version +1
    WHERE id = $4 AND version = $5
    RETURNING version
    `

	args := []any{
		user.Name,
		user.Password.hash,
		user.Level,
		user.ID,
		user.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&user.Version)
	if err != nil {
		switch {
		case err.Error() == `pq: un valore chiave duplicato viola il vincolo univoco "users_name_key"` ||
			err.Error() == `pq: duplicate key value violates unique constraint "users_name_key"`:
			return ErrDuplicateName
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}

	return nil
}
