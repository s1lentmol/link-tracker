package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/pkg/grpcx"
	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/shared/pb"
)

type BotClient struct {
	conn    *grpc.ClientConn
	client  pb.BotServiceClient
	timeout time.Duration
}

func NewBotClient(addr string, timeout time.Duration, logger *slog.Logger) (*BotClient, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(grpcx.UnaryClientLogger(logger, addr)),
	)
	if err != nil {
		return nil, fmt.Errorf("create bot grpc client: %w", err)
	}

	return &BotClient{conn: conn, client: pb.NewBotServiceClient(conn), timeout: timeout}, nil
}

func (c *BotClient) Close() error {
	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("close grpc client connection: %w", err)
	}
	return nil
}

func (c *BotClient) SendUpdate(id int64, url string, description string, chatIDs []int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	_, err := c.client.SendUpdate(ctx, &pb.LinkUpdate{
		Id:          id,
		Url:         url,
		Description: description,
		TgChatIds:   chatIDs,
	})
	if err != nil {
		return fmt.Errorf("send update: %w", err)
	}

	return nil
}
