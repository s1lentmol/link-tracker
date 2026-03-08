package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/grpcx"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/shared/pb"
)

type ScrapperClient struct {
	conn    *grpc.ClientConn
	client  pb.ScrapperServiceClient
	timeout time.Duration
}

func NewScrapperClient(addr string, timeout time.Duration, logger *slog.Logger) (*ScrapperClient, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(grpcx.UnaryClientLogger(logger, addr)),
	)
	if err != nil {
		return nil, fmt.Errorf("create grpc client: %w", err)
	}

	return &ScrapperClient{
		conn:    conn,
		client:  pb.NewScrapperServiceClient(conn),
		timeout: timeout,
	}, nil
}

func (c *ScrapperClient) Close() error {
	return c.conn.Close()
}

func (c *ScrapperClient) RegisterChat(chatID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	_, err := c.client.RegisterChat(ctx, &pb.RegisterChatRequest{ChatId: chatID})
	if err != nil {
		return fmt.Errorf("register chat: %w", err)
	}

	return nil
}

func (c *ScrapperClient) DeleteChat(chatID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	_, err := c.client.DeleteChat(ctx, &pb.DeleteChatRequest{ChatId: chatID})
	if err != nil {
		return fmt.Errorf("delete chat: %w", err)
	}

	return nil
}

func (c *ScrapperClient) AddLink(chatID int64, link string, tags []string, filters []string) (*pb.LinkResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	resp, err := c.client.AddLink(ctx, &pb.AddLinkRequest{
		ChatId:  chatID,
		Link:    link,
		Tags:    tags,
		Filters: filters,
	})
	if err != nil {
		return nil, fmt.Errorf("add link: %w", err)
	}

	return resp, nil
}

func (c *ScrapperClient) RemoveLink(chatID int64, link string) (*pb.LinkResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	resp, err := c.client.RemoveLink(ctx, &pb.RemoveLinkRequest{ChatId: chatID, Link: link})
	if err != nil {
		return nil, fmt.Errorf("remove link: %w", err)
	}

	return resp, nil
}

func (c *ScrapperClient) ListLinks(chatID int64) (*pb.ListLinksResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	resp, err := c.client.ListLinks(ctx, &pb.ListLinksRequest{ChatId: chatID})
	if err != nil {
		return nil, fmt.Errorf("list links: %w", err)
	}

	return resp, nil
}
