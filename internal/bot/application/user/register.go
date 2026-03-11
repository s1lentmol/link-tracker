package user

import (
	"fmt"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type UserRepo interface {
	Save(user domain.User) error
	FindByChatID(chatID int64) (*domain.User, error)
	Exists(chatID int64) (bool, error)
}

type UseCase struct {
	repo UserRepo
}

func NewUseCase(repo UserRepo) *UseCase {
	return &UseCase{repo: repo}
}

func (u *UseCase) RegisterUser(chatID int64, username string) (bool, error) {
	exists, err := u.repo.Exists(chatID)
	if err != nil {
		return false, fmt.Errorf("save user: %w", err)
	}

	if exists {
		return false, nil
	}

	user := domain.User{
		ChatID:   chatID,
		Username: username,
	}

	if err := u.repo.Save(user); err != nil {
		return false, fmt.Errorf("check user exists: %w", err)
	}

	return true, nil
}
