package message

import "github.com/HalvaPovidlo/messenger/internal/pkg/message"

// legacyMessages is the read-only format used to import old chat histories.
type legacyMessages struct {
	History []message.Message `json:"messages"`
}
