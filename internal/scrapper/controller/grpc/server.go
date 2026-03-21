package grpc

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/apperr"
	appstorage "gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/application/storage"
	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/application/tracker"
	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/shared/pb"
)

type Server struct {
	pb.UnimplementedScrapperServiceServer
	repo    appstorage.Repository
	tracker *tracker.Service
}

func NewServer(repo appstorage.Repository, trackerSvc *tracker.Service) *Server {
	return &Server{repo: repo, tracker: trackerSvc}
}

func (s *Server) RegisterChat(ctx context.Context, req *pb.RegisterChatRequest) (*pb.RegisterChatResponse, error) {
	if req.GetChatId() == 0 {
		return nil, statusWrappedError(codes.InvalidArgument, "chat_id is required")
	}

	err := s.repo.RegisterChat(ctx, req.GetChatId())
	if err != nil {
		if errors.Is(err, apperr.ErrChatExists) {
			return nil, statusWrappedError(codes.AlreadyExists, "chat already exists")
		}
		return nil, statusWrappedError(codes.Internal, err.Error())
	}

	return &pb.RegisterChatResponse{}, nil
}

func (s *Server) DeleteChat(ctx context.Context, req *pb.DeleteChatRequest) (*pb.DeleteChatResponse, error) {
	if req.GetChatId() == 0 {
		return nil, statusWrappedError(codes.InvalidArgument, "chat_id is required")
	}

	err := s.repo.DeleteChat(ctx, req.GetChatId())
	if err != nil {
		if errors.Is(err, apperr.ErrChatNotFound) {
			return nil, statusWrappedError(codes.NotFound, "chat not found")
		}
		return nil, statusWrappedError(codes.Internal, err.Error())
	}

	return &pb.DeleteChatResponse{}, nil
}

func (s *Server) ListLinks(ctx context.Context, req *pb.ListLinksRequest) (*pb.ListLinksResponse, error) {
	if req.GetChatId() == 0 {
		return nil, statusWrappedError(codes.InvalidArgument, "chat_id is required")
	}

	subs, err := s.repo.ListLinks(ctx, req.GetChatId())
	if err != nil {
		if errors.Is(err, apperr.ErrChatNotFound) {
			return nil, statusWrappedError(codes.NotFound, "chat not found")
		}
		return nil, statusWrappedError(codes.Internal, err.Error())
	}

	links := make([]*pb.LinkResponse, 0, len(subs))
	for _, sub := range subs {
		links = append(links, &pb.LinkResponse{
			Id:      sub.ID,
			Url:     sub.URL,
			Tags:    sub.Tags,
			Filters: sub.Filters,
		})
	}

	return &pb.ListLinksResponse{Links: links, Size: int32(len(links))}, nil
}

func (s *Server) AddLink(ctx context.Context, req *pb.AddLinkRequest) (*pb.LinkResponse, error) {
	if req.GetChatId() == 0 || req.GetLink() == "" {
		return nil, statusWrappedError(codes.InvalidArgument, "chat_id and link are required")
	}
	if err := s.tracker.ValidateURL(req.GetLink()); err != nil {
		if errors.Is(err, apperr.ErrUnsupportedLink) || errors.Is(err, apperr.ErrInvalidLink) {
			return nil, statusWrappedError(codes.InvalidArgument, "invalid or unsupported link")
		}
		return nil, statusWrappedError(codes.InvalidArgument, err.Error())
	}

	sub, err := s.repo.AddLink(ctx, req.GetChatId(), req.GetLink(), req.GetTags(), req.GetFilters())
	if err != nil {
		switch {
		case errors.Is(err, apperr.ErrChatNotFound):
			return nil, statusWrappedError(codes.NotFound, "chat not found")
		case errors.Is(err, apperr.ErrLinkExists):
			return nil, statusWrappedError(codes.AlreadyExists, "link already tracked")
		default:
			return nil, statusWrappedError(codes.Internal, err.Error())
		}
	}

	return &pb.LinkResponse{Id: sub.ID, Url: sub.URL, Tags: sub.Tags, Filters: sub.Filters}, nil
}

func (s *Server) RemoveLink(ctx context.Context, req *pb.RemoveLinkRequest) (*pb.LinkResponse, error) {
	if req.GetChatId() == 0 || req.GetLink() == "" {
		return nil, statusWrappedError(codes.InvalidArgument, "chat_id and link are required")
	}
	if err := s.tracker.ValidateURL(req.GetLink()); err != nil {
		return nil, statusWrappedError(codes.InvalidArgument, fmt.Sprintf("invalid link: %v", err))
	}

	sub, err := s.repo.RemoveLink(ctx, req.GetChatId(), req.GetLink())
	if err != nil {
		switch {
		case errors.Is(err, apperr.ErrChatNotFound), errors.Is(err, apperr.ErrLinkNotFound):
			return nil, statusWrappedError(codes.NotFound, "chat or link not found")
		default:
			return nil, statusWrappedError(codes.Internal, err.Error())
		}
	}

	return &pb.LinkResponse{Id: sub.ID, Url: sub.URL, Tags: sub.Tags, Filters: sub.Filters}, nil
}

func statusWrappedError(code codes.Code, message string) error {
	return fmt.Errorf("grpc status: %w", status.Error(code, message))
}
