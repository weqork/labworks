package user

import (
	"context"
	"time"

	"github.com/HalvaPovidlo/messenger/internal/pkg/user"
)

func (s *service) Users() ([]user.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := s.db.QueryContext(ctx, "SELECT id, login, name, surname FROM messenger_accounts ORDER BY login")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []user.User{}
	for rows.Next() {
		var profile user.User
		if err := rows.Scan(&profile.ID, &profile.Login, &profile.Name, &profile.Surname); err != nil {
			return nil, err
		}
		users = append(users, profile)
	}
	return users, rows.Err()
}
