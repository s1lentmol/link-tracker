package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRepository_AccessType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		accessType string
		wantErr    bool
	}{
		{name: "sql", accessType: "sql", wantErr: false},
		{name: "squirrel", accessType: "squirrel", wantErr: false},
		{name: "invalid", accessType: "invalid", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, err := newRepository(tt.accessType, nil)
			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, repo)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, repo)
		})
	}
}
