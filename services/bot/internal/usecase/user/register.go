package user

import "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/services/bot/internal/domain"

type UserRepo interface {
	Save(user domain.User) error
	FindByChatID(chatID int64) (*domain.User, error)
	Exists(chatID int64) (bool, error)
}

type UserUseCase struct {
	repo UserRepo
}

func NewUserUseCase(repo UserRepo) *UserUseCase {
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

	user := domain.User{
		ChatID:   chatID,
		Username: username,
	}

	if err := u.repo.Save(user); err != nil {
		return false, err
	}

	return true, nil
}
