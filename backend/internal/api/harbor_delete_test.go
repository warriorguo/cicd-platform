package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/warriorguo/cicd-platform/backend/internal/models"
	"github.com/warriorguo/cicd-platform/backend/pkg/config"
)

// MockHarborServer creates a mock Harbor server for testing
func MockHarborServer() *httptest.Server {
	mux := http.NewServeMux()
	
	// Mock Harbor API endpoints
	mux.HandleFunc("/api/v2.0/projects/library/repositories/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			// Mock response for listing repositories
			repos := []map[string]interface{}{
				{
					"name": "library/testapp1",
					"artifact_count": 3,
					"creation_time": "2026-01-01T00:00:00.000Z",
				},
				{
					"name": "library/testapp2", 
					"artifact_count": 2,
					"creation_time": "2026-01-02T00:00:00.000Z",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(repos)
		}
	})
	
	// Mock artifact deletion endpoint
	mux.HandleFunc("/api/v2.0/projects/library/repositories/testapp1/artifacts/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			// Mock artifacts list
			artifacts := []map[string]interface{}{
				{
					"digest": "sha256:abc123",
					"tags": []map[string]interface{}{
						{"name": "v1.0.0"},
						{"name": "latest"},
					},
				},
				{
					"digest": "sha256:def456",
					"tags": []map[string]interface{}{
						{"name": "v1.0.1"},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(artifacts)
		case "DELETE":
			// Mock successful deletion
			w.WriteHeader(http.StatusOK)
		}
	})
	
	// Mock repository deletion endpoint
	mux.HandleFunc("/api/v2.0/projects/library/repositories/testapp1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusOK)
		}
	})
	
	return httptest.NewServer(mux)
}

// TestDeleteHarborImage tests the Harbor image deletion functionality
func TestDeleteHarborImage(t *testing.T) {
	tests := []struct {
		name          string
		app           *models.App
		imageTag      string
		expectedError bool
		description   string
	}{
		{
			name: "Successful deletion",
			app: &models.App{
				Name:         "testapp1",
				RegistryRepo: "local-harbor.playquota.com/library/testapp1",
			},
			imageTag:      "local-harbor.playquota.com/library/testapp1:v1.0.0",
			expectedError: false,
			description:   "Should successfully delete Harbor image",
		},
		{
			name: "Invalid registry repo format",
			app: &models.App{
				Name:         "testapp2",
				RegistryRepo: "invalid-format", // Less than 3 parts after splitting by /
			},
			imageTag:      "invalid-format",
			expectedError: true,
			description:   "Should fail with invalid registry repo format",
		},
		{
			name: "Empty registry repo with invalid image tag",
			app: &models.App{
				Name:         "testapp3",
				RegistryRepo: "", // Empty registry repo forces fallback to imageTag parsing
			},
			imageTag:      "invalid", // Invalid format (no colon)
			expectedError: true,
			description:   "Should fail with invalid image tag format when registry repo is empty",
		},
	}
	
	// Start mock Harbor server
	mockServer := MockHarborServer()
	defer mockServer.Close()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock config
			// Use mockServer.URL for the URL field so the handler uses HTTP for the test
			cfg := &config.Config{
				Harbor: config.HarborConfig{
					Enabled:  true,
					Endpoint: strings.TrimPrefix(mockServer.URL, "http://"),
					URL:      mockServer.URL, // This includes http:// so handler will use it directly
					Project:  "library",
					Username: "admin",
					Password: "Harbor12345",
				},
			}
			
			// Create a handler instance with mock config
			handler := &Handler{
				config: cfg,
			}
			
			// Test the deleteHarborImage method
			err := handler.deleteHarborImage(tt.app, tt.imageTag)
			
			if tt.expectedError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

// TestExtractImageNameFromRegistry tests the logic for extracting image name from registry repo
func TestExtractImageNameFromRegistry(t *testing.T) {
	tests := []struct {
		registryRepo string
		expected     string
		shouldError  bool
	}{
		{
			registryRepo: "local-harbor.playquota.com/library/testapp1",
			expected:     "testapp1",
			shouldError:  false,
		},
		{
			registryRepo: "harbor.example.com/project/myapp",
			expected:     "myapp", 
			shouldError:  false,
		},
		{
			registryRepo: "invalid-format",
			expected:     "",
			shouldError:  true,
		},
		{
			registryRepo: "",
			expected:     "",
			shouldError:  true,
		},
		{
			registryRepo: "host/project/",
			expected:     "",
			shouldError:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.registryRepo, func(t *testing.T) {
			result, err := extractImageNameFromRegistry(tt.registryRepo)
			
			if tt.shouldError {
				assert.Error(t, err)
				assert.Empty(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// Helper function to extract image name from registry repo
func extractImageNameFromRegistry(registryRepo string) (string, error) {
	if registryRepo == "" {
		return "", fmt.Errorf("empty registry repo")
	}
	
	// Split by '/' to get the parts
	parts := strings.Split(registryRepo, "/")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid registry repo format")
	}
	
	imageName := parts[len(parts)-1]
	if imageName == "" {
		return "", fmt.Errorf("empty image name")
	}
	
	return imageName, nil
}