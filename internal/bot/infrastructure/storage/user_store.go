package storage

import (
	"fmt"
	"sync"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type UserRepository struct {
	mu    sync.RWMutex
	users map[int64]domain.User
}

func NewUserRepository() *UserRepository {
	return &UserRepository{
		users: make(map[int64]domain.User),
	}
}

func (r *UserRepository) Save(user domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.users[user.ChatID] = user

	return nil
}

func (r *UserRepository) FindByChatID(chatID int64) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, ok := r.users[chatID]
	if !ok {
		return nil, fmt.Errorf("user with chatID %d not found", chatID)
	}

	return &user, nil
}

func (r *UserRepository) Exists(chatID int64) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.users[chatID]

	return ok, nil
}
