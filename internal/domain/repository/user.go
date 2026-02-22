package repository

import "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain/model"

type UserRepository interface {
	Save(user model.User) error
	FindByChatID(chatID int64) (*model.User, error)
	Exists(chatID int64) (bool, error)
}
