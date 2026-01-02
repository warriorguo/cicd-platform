package git

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGitURL(t *testing.T) {
	tests := []struct {
		name        string
		gitURL      string
		expected    *RepoInfo
		expectError bool
	}{
		{
			name:   "GitHub HTTPS URL",
			gitURL: "https://github.com/user/repo.git",
			expected: &RepoInfo{
				Provider: GitHub,
				Owner:    "user",
				Repo:     "repo",
				BaseURL:  "https://github.com",
			},
			expectError: false,
		},
		{
			name:   "GitHub HTTPS URL without .git",
			gitURL: "https://github.com/user/repo",
			expected: &RepoInfo{
				Provider: GitHub,
				Owner:    "user",
				Repo:     "repo",
				BaseURL:  "https://github.com",
			},
			expectError: false,
		},
		{
			name:   "GitHub SSH URL",
			gitURL: "git@github.com:user/repo.git",
			expected: &RepoInfo{
				Provider: GitHub,
				Owner:    "user",
				Repo:     "repo",
				BaseURL:  "https://github.com",
			},
			expectError: false,
		},
		{
			name:   "GitLab HTTPS URL",
			gitURL: "https://gitlab.com/user/repo.git",
			expected: &RepoInfo{
				Provider: GitLab,
				Owner:    "user",
				Repo:     "repo",
				BaseURL:  "https://gitlab.com",
			},
			expectError: false,
		},
		{
			name:   "Gitea HTTPS URL",
			gitURL: "https://gitea.company.com/user/repo.git",
			expected: &RepoInfo{
				Provider: Gitea,
				Owner:    "user",
				Repo:     "repo",
				BaseURL:  "https://gitea.company.com",
			},
			expectError: false,
		},
		{
			name:        "Invalid URL",
			gitURL:      "not-a-valid-url",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "URL with insufficient path",
			gitURL:      "https://github.com/user",
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseGitURL(tt.gitURL)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestDetectProvider(t *testing.T) {
	tests := []struct {
		host     string
		expected ProviderType
	}{
		{"github.com", GitHub},
		{"github.enterprise.com", GitHub},
		{"gitlab.com", GitLab},
		{"gitlab.company.com", GitLab},
		{"gitea.io", Gitea},
		{"unknown.com", Gitea}, // Default to Gitea for unknown providers
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			result := detectProvider(tt.host)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGitHubClient(t *testing.T) {
	t.Run("List Branches Success", func(t *testing.T) {
		// Create mock server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/repos/user/repo/branches", r.URL.Path)
			
			response := `[
				{"name": "main"},
				{"name": "develop"},
				{"name": "feature/test"}
			]`
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))
		}))
		defer server.Close()

		client := NewClient()
		repoInfo := &RepoInfo{
			Provider: GitHub,
			Owner:    "user",
			Repo:     "repo",
			BaseURL:  server.URL,
		}

		branches, err := client.ListBranches(context.Background(), repoInfo, "")
		
		assert.NoError(t, err)
		assert.Equal(t, []string{"main", "develop", "feature/test"}, branches)
	})

	t.Run("List Branches with Token", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "token test-token", r.Header.Get("Authorization"))
			
			response := `[{"name": "main"}]`
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))
		}))
		defer server.Close()

		client := NewClient()
		repoInfo := &RepoInfo{
			Provider: GitHub,
			Owner:    "user",
			Repo:     "repo",
			BaseURL:  server.URL,
		}

		branches, err := client.ListBranches(context.Background(), repoInfo, "test-token")
		
		assert.NoError(t, err)
		assert.Equal(t, []string{"main"}, branches)
	})

	t.Run("List Branches API Error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := NewClient()
		repoInfo := &RepoInfo{
			Provider: GitHub,
			Owner:    "user",
			Repo:     "repo",
			BaseURL:  server.URL,
		}

		branches, err := client.ListBranches(context.Background(), repoInfo, "")
		
		assert.Error(t, err)
		assert.Nil(t, branches)
	})

	t.Run("Check Dockerfile Exists", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/repos/user/repo/contents/Dockerfile", r.URL.Path)
			assert.Equal(t, "main", r.URL.Query().Get("ref"))
			
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := NewClient()
		repoInfo := &RepoInfo{
			Provider: GitHub,
			Owner:    "user",
			Repo:     "repo",
			BaseURL:  server.URL,
		}

		exists, err := client.CheckDockerfile(context.Background(), repoInfo, "main", "Dockerfile", "")
		
		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("Check Dockerfile Not Found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := NewClient()
		repoInfo := &RepoInfo{
			Provider: GitHub,
			Owner:    "user",
			Repo:     "repo",
			BaseURL:  server.URL,
		}

		exists, err := client.CheckDockerfile(context.Background(), repoInfo, "main", "Dockerfile", "")
		
		assert.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestGitLabClient(t *testing.T) {
	t.Run("List Branches Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.URL.Path, "/api/v4/projects/user%2Frepo/repository/branches")
			
			response := `[
				{"name": "main"},
				{"name": "develop"}
			]`
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))
		}))
		defer server.Close()

		client := NewClient()
		repoInfo := &RepoInfo{
			Provider: GitLab,
			Owner:    "user",
			Repo:     "repo",
			BaseURL:  server.URL,
		}

		branches, err := client.ListBranches(context.Background(), repoInfo, "")
		
		assert.NoError(t, err)
		assert.Equal(t, []string{"main", "develop"}, branches)
	})

	t.Run("List Branches with Bearer Token", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			
			response := `[{"name": "main"}]`
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))
		}))
		defer server.Close()

		client := NewClient()
		repoInfo := &RepoInfo{
			Provider: GitLab,
			Owner:    "user",
			Repo:     "repo",
			BaseURL:  server.URL,
		}

		branches, err := client.ListBranches(context.Background(), repoInfo, "test-token")
		
		assert.NoError(t, err)
		assert.Equal(t, []string{"main"}, branches)
	})
}

func TestGiteaClient(t *testing.T) {
	t.Run("List Branches Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/v1/repos/user/repo/branches", r.URL.Path)
			
			response := `[
				{"name": "main"},
				{"name": "feature/new"}
			]`
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))
		}))
		defer server.Close()

		client := NewClient()
		repoInfo := &RepoInfo{
			Provider: Gitea,
			Owner:    "user",
			Repo:     "repo",
			BaseURL:  server.URL,
		}

		branches, err := client.ListBranches(context.Background(), repoInfo, "")
		
		assert.NoError(t, err)
		assert.Equal(t, []string{"main", "feature/new"}, branches)
	})
}

func TestClientIntegration(t *testing.T) {
	t.Run("Parse and Use GitHub URL", func(t *testing.T) {
		repoInfo, err := ParseGitURL("https://github.com/user/repo.git")
		require.NoError(t, err)
		
		assert.Equal(t, GitHub, repoInfo.Provider)
		assert.Equal(t, "user", repoInfo.Owner)
		assert.Equal(t, "repo", repoInfo.Repo)
		assert.Equal(t, "https://github.com", repoInfo.BaseURL)
		
		// Note: This would make a real API call, so we skip it in unit tests
		// branches, err := client.ListBranches(context.Background(), repoInfo, "")
		// In real integration tests, you would test with actual repositories
	})
}