package usecase

import (
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain/model"
)

type UserRepository interface {
	Save(user model.User) error
	FindByChatID(chatID int64) (*model.User, error)
	Exists(chatID int64) (bool, error)
}

type UserUseCase struct {
	repo UserRepository
}

func NewUserUseCase(repo UserRepository) *UserUseCase {
	return &UserUseCase{repo: repo}
}

func (u *UserUseCase) RegisterUser(chatID int64, username string) (bool, error) {
	exists, err := u.repo.Exists(chatID)
	if err != nil {
		return false, err
	}

	if exists {
		return false, nil
	}

	user := model.User{
		ChatID:   chatID,
		Username: username,
	}

	if err := u.repo.Save(user); err != nil {
		return false, err
	}

	return true, nil
}
