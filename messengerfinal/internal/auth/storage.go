package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/HalvaPovidlo/messenger/internal/pkg/user"
	"github.com/google/uuid"
)

const insertAccount = `
	INSERT INTO messenger_accounts (id, login, name, surname, password_hash)
	VALUES ($1, $2, $3, $4, $5)
`

func New(ctx context.Context, db *sql.DB, legacyDir string) (*service, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(517238105)"); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS messenger_accounts (
			id uuid PRIMARY KEY,
			login text NOT NULL UNIQUE,
			name text NOT NULL,
			surname text NOT NULL,
			password_hash bytea NOT NULL
		)
	`); err != nil {
		return nil, err
	}
	if legacyDir != "" {
		if err := importAccounts(ctx, tx, legacyDir); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &service{db: db}, nil
}

func importAccounts(ctx context.Context, tx *sql.Tx, dir string) error {
	var profiles struct {
		Data []user.User `json:"data"`
	}
	var logins struct {
		Credentials map[string]struct {
			ID       uuid.UUID `json:"id"`
			Password []byte    `json:"password"`
		} `json:"credentials"`
	}
	profilesFound, err := readLegacyJSON(filepath.Join(dir, "users.json"), &profiles)
	if err != nil {
		return err
	}
	loginsFound, err := readLegacyJSON(filepath.Join(dir, "credentials.json"), &logins)
	if err != nil {
		return err
	}
	if !profilesFound && !loginsFound {
		return nil
	}
	if profilesFound != loginsFound {
		return errors.New("importing accounts requires both users.json and credentials.json")
	}

	profilesByID := make(map[uuid.UUID]user.User, len(profiles.Data))
	for _, profile := range profiles.Data {
		profilesByID[profile.ID] = profile
	}
	for login, credentials := range logins.Credentials {
		profile, ok := profilesByID[credentials.ID]
		if !ok || profile.Login != login {
			return fmt.Errorf("legacy account %q has no matching user profile", login)
		}
		var currentID uuid.UUID
		err := tx.QueryRowContext(ctx, "SELECT id FROM messenger_accounts WHERE login = $1", login).Scan(&currentID)
		if err == nil {
			if currentID != profile.ID {
				return fmt.Errorf("legacy login %q already belongs to a different account", login)
			}
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		hash, err := hashPassword(string(credentials.Password))
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, insertAccount, profile.ID, login, profile.Name, profile.Surname, hash); err != nil {
			return err
		}
	}
	return nil
}

func readLegacyJSON(path string, target interface{}) (bool, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(content, target); err != nil {
		return false, fmt.Errorf("decode %s: %w", filepath.Base(path), err)
	}
	return true, nil
}
