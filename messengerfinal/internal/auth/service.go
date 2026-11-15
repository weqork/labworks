package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"time"

	"github.com/HalvaPovidlo/messenger/internal/pkg/user"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type service struct {
	db *sql.DB
}

// Hash the input before bcrypt so existing passwords of any length can be imported.
func passwordInput(password string) []byte {
	sum := sha256.Sum256([]byte(password))
	return sum[:]
}

func hashPassword(password string) ([]byte, error) {
	return bcrypt.GenerateFromPassword(passwordInput(password), bcrypt.DefaultCost)
}

func (s *service) Verify(login, password string) (uuid.UUID, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var id uuid.UUID
	var hash []byte
	err := s.db.QueryRowContext(ctx, "SELECT id, password_hash FROM messenger_accounts WHERE login = $1", login).Scan(&id, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	err = bcrypt.CompareHashAndPassword(hash, passwordInput(password))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	return id, true, nil
}

func (s *service) Register(name, surname, login, password string) error {
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	profile := user.New(login, name, surname)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = s.db.ExecContext(ctx, insertAccount, profile.ID, login, name, surname, hash)
	return err
}
