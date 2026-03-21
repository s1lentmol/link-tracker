package grpc

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/shared/pb"
)

type MessageSender interface {
	SendMessage(chatID int64, text string) error
}

type BotUpdatesServer struct {
	pb.UnimplementedBotServiceServer
	sender MessageSender
	logger *slog.Logger
}

func NewBotUpdatesServer(sender MessageSender, logger *slog.Logger) *BotUpdatesServer {
	return &BotUpdatesServer{sender: sender, logger: logger}
}

func (s *BotUpdatesServer) SendUpdate(_ context.Context, update *pb.LinkUpdate) (*pb.SendUpdateResponse, error) {
	if update == nil {
		return nil, errors.New("empty update")
	}

	for _, chatID := range update.GetTgChatIds() {
		text := buildUpdateMessage(update)
		if err := s.sender.SendMessage(chatID, text); err != nil {
			s.logger.Error("failed to send update message",
				slog.Int64("chat_id", chatID),
				slog.Int64("link_id", update.GetId()),
				slog.String("url", update.GetUrl()),
				slog.String("error", err.Error()),
			)
		}
	}

	s.logger.Info("update delivered",
		slog.Int64("link_id", update.GetId()),
		slog.String("url", update.GetUrl()),
		slog.Int("chat_count", len(update.GetTgChatIds())),
	)

	return &pb.SendUpdateResponse{}, nil
}

func buildUpdateMessage(update *pb.LinkUpdate) string {
	var sb strings.Builder
	sb.WriteString("Обнаружено обновление по ссылке\n")
	sb.WriteString(update.GetUrl())
	if desc := strings.TrimSpace(update.GetDescription()); desc != "" {
		sb.WriteString("\n\n")
		sb.WriteString(desc)
	}
	return sb.String()
}
