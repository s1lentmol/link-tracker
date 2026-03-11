package stackoverflow_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stackclient "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/http/stackoverflow"
)

func TestClient_QuestionUpdatedAt(t *testing.T) {
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
			body:       `{"items":[{"last_activity_date":1741428000}]}`,
			wantTime:   time.Unix(1741428000, 0).UTC(),
		},
		{
			name:           "non-2xx статус",
			statusCode:     http.StatusBadGateway,
			body:           `{"error":"bad gateway"}`,
			wantErr:        true,
			wantErrContain: "stackoverflow status",
		},
		{
			name:           "некорректный json",
			statusCode:     http.StatusOK,
			body:           `{"items":`,
			wantErr:        true,
			wantErrContain: "decode body",
		},
		{
			name:           "пустой items",
			statusCode:     http.StatusOK,
			body:           `{"items":[]}`,
			wantErr:        true,
			wantErrContain: "missing last_activity_date",
		},
		{
			name:           "нулевая last_activity_date",
			statusCode:     http.StatusOK,
			body:           `{"items":[{"last_activity_date":0}]}`,
			wantErr:        true,
			wantErrContain: "missing last_activity_date",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/questions/123", r.URL.Path)
				assert.Equal(t, "stackoverflow", r.URL.Query().Get("site"))
				w.WriteHeader(tt.statusCode)
				_, _ = fmt.Fprint(w, tt.body)
			}))
			defer ts.Close()

			client := stackclient.New(ts.URL, ts.Client())
			got, err := client.QuestionUpdatedAt(context.Background(), "123")

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
