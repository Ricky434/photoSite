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
	Exists(id int) (bool, error)
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
    INSERT INTO users (name, password_hash)
    VALUES ($1, $2)
    RETURNING id, created_at, version
    `

	args := []any{user.Name, user.Password.hash}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&user.ID, &user.CreatedAt, &user.Version)
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
    SELECT id, created_at, name, password_hash, version
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

func (m *UserModel) Exists(id int) (bool, error) {
	if _, err := m.GetById(id); err != nil {
		if errors.Is(err, ErrRecordNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (m *UserModel) Update(user *User) error {
	query := `
    UPDATE users
    SET name = $1, password_hash = $2, version = version +1
    WHERE id = $3 AND version = $4
    RETURNING version
    `

	args := []any{
		user.Name,
		user.Password.hash,
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
