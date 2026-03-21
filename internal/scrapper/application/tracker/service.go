package tracker

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/apperr"
	appstorage "gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/application/storage"
	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/infrastructure/http/github"
	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/infrastructure/http/stackoverflow"
)

var stackOverflowQuestionPath = regexp.MustCompile(`^/questions/(\d+)(/.*)?$`)

const stackOverflowMinMatches = 2

type BotNotifier interface {
	SendUpdate(id int64, url string, description string, chatIDs []int64) error
}

type Service struct {
	repo     appstorage.Repository
	github   *github.Client
	stack    *stackoverflow.Client
	notifier BotNotifier
	logger   *slog.Logger
}

func New(repo appstorage.Repository, githubClient *github.Client, stackClient *stackoverflow.Client, notifier BotNotifier, logger *slog.Logger) *Service {
	return &Service{repo: repo, github: githubClient, stack: stackClient, notifier: notifier, logger: logger}
}

func (s *Service) CheckUpdates(ctx context.Context) {
	const pageSize = 200
	for offset := 0; ; offset += pageSize {
		resources, err := s.repo.ListResourcesPage(ctx, pageSize, offset)
		if err != nil {
			s.logger.Error("failed to fetch resources page",
				slog.Int("limit", pageSize),
				slog.Int("offset", offset),
				slog.String("error", err.Error()),
			)
			return
		}

		if len(resources) == 0 {
			return
		}

		for _, res := range resources {
			updatedAt, resolveErr := s.resolveUpdatedAt(ctx, res.URL)
			if resolveErr != nil {
				s.logger.Warn("failed to fetch resource update timestamp",
					slog.String("url", res.URL),
					slog.String("error", resolveErr.Error()),
				)
				continue
			}

			if !res.LastUpdate.IsZero() && !updatedAt.After(res.LastUpdate) {
				continue
			}

			if res.LastUpdate.IsZero() {
				if setErr := s.repo.SetLastUpdateByLinkID(ctx, res.ID, updatedAt); setErr != nil {
					s.logger.Error("failed to save initial last_update",
						slog.Int64("link_id", res.ID),
						slog.String("url", res.URL),
						slog.String("error", setErr.Error()),
					)
				}
				continue
			}

			if setErr := s.repo.SetLastUpdateByLinkID(ctx, res.ID, updatedAt); setErr != nil {
				s.logger.Error("failed to update last_update",
					slog.Int64("link_id", res.ID),
					slog.String("url", res.URL),
					slog.String("error", setErr.Error()),
				)
				continue
			}

			err = s.notifier.SendUpdate(res.ID, res.URL, "В ресурсе появились изменения", res.ChatIDs)
			if err != nil {
				s.logger.Error("failed to send update to bot",
					slog.Int64("link_id", res.ID),
					slog.String("url", res.URL),
					slog.String("error", err.Error()),
				)
				continue
			}

			s.logger.Info("update sent",
				slog.Int64("link_id", res.ID),
				slog.String("url", res.URL),
				slog.Int("chat_count", len(res.ChatIDs)),
			)
		}
	}
}

func (s *Service) ValidateURL(raw string) error {
	_, err := parseTarget(raw)
	return err
}

func (s *Service) resolveUpdatedAt(ctx context.Context, raw string) (time.Time, error) {
	target, err := parseTarget(raw)
	if err != nil {
		return time.Time{}, err
	}

	switch target.kind {
	case "github":
		updatedAt, repoErr := s.github.GetRepoUpdatedAt(ctx, target.owner, target.repo)
		if repoErr != nil {
			return time.Time{}, fmt.Errorf("github updated_at: %w", repoErr)
		}
		return updatedAt, nil
	case "stackoverflow":
		updatedAt, questionErr := s.stack.QuestionUpdatedAt(ctx, target.questionID)
		if questionErr != nil {
			return time.Time{}, fmt.Errorf("stackoverflow updated_at: %w", questionErr)
		}
		return updatedAt, nil
	default:
		return time.Time{}, apperr.ErrUnsupportedLink
	}
}

type target struct {
	kind       string
	owner      string
	repo       string
	questionID string
}

func parseTarget(raw string) (*target, error) {
	u, err := url.ParseRequestURI(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("%w: parse url", apperr.ErrInvalidLink)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("%w: unsupported scheme", apperr.ErrInvalidLink)
	}

	host := strings.ToLower(u.Host)
	switch host {
	case "github.com":
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("%w: invalid github path", apperr.ErrInvalidLink)
		}
		return &target{kind: "github", owner: parts[0], repo: parts[1]}, nil
	case "stackoverflow.com":
		m := stackOverflowQuestionPath.FindStringSubmatch(u.Path)
		if len(m) < stackOverflowMinMatches {
			return nil, fmt.Errorf("%w: invalid stackoverflow path", apperr.ErrInvalidLink)
		}
		return &target{kind: "stackoverflow", questionID: m[1]}, nil
	default:
		return nil, apperr.ErrUnsupportedLink
	}
}
