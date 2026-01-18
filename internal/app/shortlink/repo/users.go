package repo

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var ErrUserNotFound = errors.New("user not found")
var ErrUserAlreadyExists = errors.New("Username already exists")
var ErrInvalidUsername = errors.New("Username is not allowed")
var ErrInvalidPassword = errors.New("Password is not allowed")

type UsersRepo struct {
	db *pgxpool.Pool
}

func NewUsersRepo(db *pgxpool.Pool) *UsersRepo {
	return &UsersRepo{db: db}
}

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	Role         string
}

func (u *UsersRepo) FindByUsername(ctx context.Context, username string) (User, error) {
	username = strings.TrimSpace(username)
	dbctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	row := u.db.QueryRow(dbctx, "SELECT id, username, password_hash, role FROM users WHERE username=$1 LIMIT 1", username)
	var user User
	if err := row.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		slog.Error(err.Error())
		return User{}, err
	}
	return user, nil
}

func (u *UsersRepo) RegistUser(ctx context.Context, name string, password string) (int64, error) {
	name = strings.TrimSpace(name)
	if len(name) < 3 || len(name) > 32 {
		return -1, ErrInvalidUsername
	}
	if len(password) < 8 || len(password) > 72 {
		return -1, ErrInvalidPassword
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		slog.Error(err.Error())
		return -1, err
	}
	dbctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var id int64
	if err := u.db.
		QueryRow(dbctx, "Insert INTO users (username,password_hash,role) VALUES ($1,$2,'user') ON CONFLICT (username) DO NOTHING RETURNING id", name, string(passwordHash)).
		Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return -1, ErrUserAlreadyExists
		}
		slog.Error(err.Error())
		return -1, err
	}

	return id, nil
}
