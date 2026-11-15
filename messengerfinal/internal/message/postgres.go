package message

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/HalvaPovidlo/messenger/internal/pkg/message"
	"github.com/google/uuid"
)

type PostgresStorage struct {
	db *sql.DB
}

const insertMessage = "INSERT INTO messages (chat_id, sender_id, body) VALUES ($1, $2, $3)"

func NewPostgresStorage(ctx context.Context, db *sql.DB, legacyDir string) (*PostgresStorage, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Serialize schema setup and imports when multiple servers start together.
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(517238104)"); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS messages (
			id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
			chat_id text NOT NULL,
			sender_id uuid NOT NULL,
			body text NOT NULL
		);
		CREATE INDEX IF NOT EXISTS messages_chat_id_id ON messages (chat_id, id);
		CREATE TABLE IF NOT EXISTS imported_message_chats (chat_id text PRIMARY KEY);
	`); err != nil {
		return nil, err
	}
	if legacyDir != "" {
		if err := importLegacyChats(ctx, tx, legacyDir); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &PostgresStorage{db: db}, nil
}

func (s *PostgresStorage) Message(msg message.Message, chatID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.db.ExecContext(ctx, insertMessage, chatID, msg.ID, msg.Text)
	return err
}

func (s *PostgresStorage) History(chatID string) ([]message.Message, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `
		SELECT sender_id, body FROM messages WHERE chat_id = $1 ORDER BY id
	`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []message.Message
	for rows.Next() {
		var msg message.Message
		if err := rows.Scan(&msg.ID, &msg.Text); err != nil {
			return nil, err
		}
		history = append(history, msg)
	}
	return history, rows.Err()
}

func importLegacyChats(ctx context.Context, tx *sql.Tx, dir string) error {
	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, file := range files {
		name := file.Name()
		if !file.Type().IsRegular() || !strings.HasSuffix(name, ".json") {
			continue
		}
		chatID := strings.TrimSuffix(name, ".json")
		left, right, ok := strings.Cut(chatID, "_")
		if !ok || len(left) != 36 || len(right) != 36 {
			continue
		}
		first, firstErr := uuid.Parse(left)
		second, secondErr := uuid.Parse(right)
		if firstErr != nil || secondErr != nil || string(buildKey(first, second)) != chatID {
			continue
		}

		result, err := tx.ExecContext(ctx, `
			INSERT INTO imported_message_chats (chat_id) VALUES ($1)
			ON CONFLICT DO NOTHING
		`, chatID)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if count == 0 {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		var messages legacyMessages
		if err := json.Unmarshal(content, &messages); err != nil {
			return fmt.Errorf("read legacy chat %s: %w", name, err)
		}
		for _, msg := range messages.History {
			if _, err := tx.ExecContext(ctx, insertMessage, chatID, msg.ID, msg.Text); err != nil {
				return fmt.Errorf("import legacy chat %s: %w", name, err)
			}
		}
	}
	return nil
}
