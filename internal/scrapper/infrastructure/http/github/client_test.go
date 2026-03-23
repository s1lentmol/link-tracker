package github_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ghclient "gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/infrastructure/http/github"
)

func TestClient_GetRepoUpdatedAt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		statusCode     int
		body           string
		wantErr        bool
		wantErrContain string
		wantTime       time.Time
	}{
		{
			name:       "успешный ответ",
			statusCode: http.StatusOK,
			body:       `{"updated_at":"2026-03-08T10:00:00Z"}`,
			wantTime:   time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC),
		},
		{
			name:           "non-2xx статус",
			statusCode:     http.StatusInternalServerError,
			body:           `{"message":"error"}`,
			wantErr:        true,
			wantErrContain: "github status",
		},
		{
			name:           "некорректный json",
			statusCode:     http.StatusOK,
			body:           `{"updated_at":`,
			wantErr:        true,
			wantErrContain: "decode body",
		},
		{
			name:           "пустой updated_at",
			statusCode:     http.StatusOK,
			body:           `{"updated_at":""}`,
			wantErr:        true,
			wantErrContain: "missing updated_at",
		},
		{
			name:           "невалидный формат updated_at",
			statusCode:     http.StatusOK,
			body:           `{"updated_at":"not-a-time"}`,
			wantErr:        true,
			wantErrContain: "parse updated_at",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/repos/owner/repo", r.URL.Path)
				w.WriteHeader(tt.statusCode)
				_, _ = fmt.Fprint(w, tt.body)
			}))
			defer ts.Close()

			client := ghclient.New(ts.URL, ts.Client())
			got, err := client.GetRepoUpdatedAt(context.Background(), "owner", "repo")

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContain != "" {
					assert.Contains(t, err.Error(), tt.wantErrContain)
				}
				return
			}

			require.NoError(t, err)
			assert.True(t, got.Equal(tt.wantTime), "ожидали %s, получили %s", tt.wantTime, got)
		})
	}
}
