package message

import (
	"github.com/HalvaPovidlo/messenger/internal/pkg/message"
	"github.com/google/uuid"
)

type database interface {
	History(chatID string) ([]message.Message, error)
	Message(msg message.Message, chatID string) error
}

type service struct {
	storage database
}

func New(database database) *service {
	return &service{storage: database}
}

func (s *service) Message(from, to uuid.UUID, text string) error {
	msg := message.Message{ID: from, Text: text}
	return s.storage.Message(msg, buildKey(from, to))
}

func (s *service) History(person1, person2 uuid.UUID) ([]message.Message, error) {
	return s.storage.History(buildKey(person1, person2))
}

func buildKey(from, to uuid.UUID) string {
	f := from.String()
	t := to.String()
	if f > t {
		return f + "_" + t
	}
	return t + "_" + f
}
