package commands

import (
	"testing"

	"github.com/stretchr/testify/require"
	evaluations "github.com/wolfeidau/mcp-evals"
)

func TestFilterEvals(t *testing.T) {
	evals := []evaluations.Eval{
		{Name: "auth_basic"},
		{Name: "auth_token"},
		{Name: "user_create"},
		{Name: "user_delete"},
		{Name: "admin_auth"},
		{Name: "troubleshoot_network"},
		{Name: "troubleshoot_service"},
	}

	tests := []struct {
		name     string
		pattern  string
		expected []string
		wantErr  bool
	}{
		{
			name:     "match prefix",
			pattern:  "^auth",
			expected: []string{"auth_basic", "auth_token"},
		},
		{
			name:     "match suffix",
			pattern:  "auth$",
			expected: []string{"admin_auth"},
		},
		{
			name:     "match multiple",
			pattern:  "auth|user",
			expected: []string{"auth_basic", "auth_token", "user_create", "user_delete", "admin_auth"},
		},
		{
			name:     "match substring",
			pattern:  "token",
			expected: []string{"auth_token"},
		},
		{
			name:     "match all troubleshoot evals",
			pattern:  "troubleshoot_.*",
			expected: []string{"troubleshoot_network", "troubleshoot_service"},
		},
		{
			name:     "no matches",
			pattern:  "nonexistent",
			expected: nil,
		},
		{
			name:     "exact match",
			pattern:  "^user_create$",
			expected: []string{"user_create"},
		},
		{
			name:     "case sensitive",
			pattern:  "Auth",
			expected: nil, // No matches because lowercase
		},
		{
			name:    "invalid regex",
			pattern: "[invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert := require.New(t)

			result, err := filterEvals(evals, tt.pattern)

			if tt.wantErr {
				assert.Error(err)
				return
			}

			assert.NoError(err)

			var names []string
			for _, e := range result {
				names = append(names, e.Name)
			}
			assert.Equal(tt.expected, names)
		})
	}
}

func TestFilterEvals_EmptyInput(t *testing.T) {
	t.Parallel()
	assert := require.New(t)

	evals := []evaluations.Eval{}

	result, err := filterEvals(evals, ".*")
	assert.NoError(err)
	assert.Empty(result)
}

func TestFilterEvals_MatchAll(t *testing.T) {
	t.Parallel()
	assert := require.New(t)

	evals := []evaluations.Eval{
		{Name: "test1"},
		{Name: "test2"},
		{Name: "test3"},
	}

	result, err := filterEvals(evals, ".*")
	assert.NoError(err)
	assert.Len(result, 3)
}

func TestFilterEvals_ComplexPattern(t *testing.T) {
	t.Parallel()
	assert := require.New(t)

	evals := []evaluations.Eval{
		{Name: "api_v1_users"},
		{Name: "api_v2_users"},
		{Name: "api_v1_posts"},
		{Name: "api_v2_posts"},
		{Name: "internal_health"},
	}

	// Match all v2 endpoints
	result, err := filterEvals(evals, "api_v2_.*")
	assert.NoError(err)
	assert.Len(result, 2)

	var names []string
	for _, e := range result {
		names = append(names, e.Name)
	}
	assert.Equal([]string{"api_v2_users", "api_v2_posts"}, names)
}
