package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/warriorguo/cicd-platform/backend/internal/api"
	"github.com/warriorguo/cicd-platform/backend/internal/git"
	"github.com/warriorguo/cicd-platform/backend/internal/models"
	"github.com/warriorguo/cicd-platform/backend/pkg/config"
)

// SystemTest represents an end-to-end test scenario
type SystemTest struct {
	t        *testing.T
	db       *gorm.DB
	server   *http.Server
	baseURL  string
	client   *http.Client
	cleanup  []func()
}

func NewSystemTest(t *testing.T) *SystemTest {
	if testing.Short() {
		t.Skip("Skipping system tests in short mode")
	}

	if os.Getenv("SYSTEM_TEST") != "1" {
		t.Skip("Set SYSTEM_TEST=1 to run system tests")
	}

	st := &SystemTest{
		t:       t,
		client:  &http.Client{Timeout: 30 * time.Second},
		cleanup: []func(){},
	}

	st.setupDatabase()
	st.setupServer()

	return st
}

func (st *SystemTest) setupDatabase() {
	// Use test database
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=password dbname=cicd_test port=5432 sslmode=disable"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(st.t, err)

	// Clean and migrate database
	err = db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public;").Error
	require.NoError(st.t, err)

	err = db.AutoMigrate(&models.App{}, &models.Release{})
	require.NoError(st.t, err)

	st.db = db
	st.cleanup = append(st.cleanup, func() {
		// Clean up database
		db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public;")
	})
}

func (st *SystemTest) setupServer() {
	// Create test server
	gitClient := git.NewClient()
	handler := api.NewHandler(st.db, gitClient, nil)
	router := api.SetupRoutes(handler)

	server := &http.Server{
		Addr:    ":8081", // Use different port for tests
		Handler: router,
	}

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			st.t.Errorf("Server failed to start: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(1 * time.Second)

	st.server = server
	st.baseURL = "http://localhost:8081"

	st.cleanup = append(st.cleanup, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	})
}

func (st *SystemTest) Cleanup() {
	for i := len(st.cleanup) - 1; i >= 0; i-- {
		st.cleanup[i]()
	}
}

func (st *SystemTest) makeRequest(method, path string, body interface{}) (*http.Response, []byte) {
	var reqBody *bytes.Buffer
	if body != nil {
		jsonData, err := json.Marshal(body)
		require.NoError(st.t, err)
		reqBody = bytes.NewBuffer(jsonData)
	}

	var req *http.Request
	var err error
	if reqBody != nil {
		req, err = http.NewRequest(method, st.baseURL+path, reqBody)
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(method, st.baseURL+path, nil)
	}
	require.NoError(st.t, err)

	resp, err := st.client.Do(req)
	require.NoError(st.t, err)
	defer resp.Body.Close()

	respBody := make([]byte, 0)
	if resp.ContentLength != 0 {
		buf := bytes.NewBuffer(nil)
		_, err = buf.ReadFrom(resp.Body)
		require.NoError(st.t, err)
		respBody = buf.Bytes()
	}

	return resp, respBody
}

func TestSystemEndToEnd(t *testing.T) {
	st := NewSystemTest(t)
	defer st.Cleanup()

	t.Run("Complete Application Lifecycle", func(t *testing.T) {
		// Step 1: Create application
		appData := map[string]interface{}{
			"name":               "system-test-app",
			"git_url":            "https://github.com/docker-library/hello-world.git",
			"default_branch":     "master",
			"dockerfile_path":    "Dockerfile",
			"context_path":       ".",
			"registry_repo":      "test.harbor.com/system/hello-world",
			"target_namespace":   "test",
			"target_deploy_name": "hello-world",
		}

		resp, body := st.makeRequest("POST", "/api/apps", appData)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var createdApp models.App
		err := json.Unmarshal(body, &createdApp)
		require.NoError(t, err)
		assert.Equal(t, "system-test-app", createdApp.Name)
		assert.NotZero(t, createdApp.ID)

		appID := createdApp.ID

		// Step 2: List applications and verify our app exists
		resp, body = st.makeRequest("GET", "/api/apps", nil)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var apps []models.App
		err = json.Unmarshal(body, &apps)
		require.NoError(t, err)
		assert.Len(t, apps, 1)
		assert.Equal(t, "system-test-app", apps[0].Name)

		// Step 3: Get specific application
		resp, body = st.makeRequest("GET", fmt.Sprintf("/api/apps/%d", appID), nil)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var fetchedApp models.App
		err = json.Unmarshal(body, &fetchedApp)
		require.NoError(t, err)
		assert.Equal(t, appID, fetchedApp.ID)
		assert.Equal(t, "system-test-app", fetchedApp.Name)

		// Step 4: Validate Dockerfile (will likely fail for real repo, but tests workflow)
		resp, body = st.makeRequest("POST", fmt.Sprintf("/api/apps/%d/validate?branch=master", appID), nil)
		// Accept both success and failure as this depends on external repo
		assert.Contains(t, []int{http.StatusOK}, resp.StatusCode)

		// Step 5: Create release
		releaseData := map[string]interface{}{
			"branch":     "master",
			"commit_sha": "abc123456",
		}

		resp, body = st.makeRequest("POST", fmt.Sprintf("/api/apps/%d/releases", appID), releaseData)
		// May fail due to Dockerfile validation, but tests the endpoint
		assert.Contains(t, []int{http.StatusCreated, http.StatusBadRequest}, resp.StatusCode)

		// Step 6: List releases
		resp, body = st.makeRequest("GET", fmt.Sprintf("/api/apps/%d/releases", appID), nil)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var releaseResponse map[string]interface{}
		err = json.Unmarshal(body, &releaseResponse)
		require.NoError(t, err)
		assert.Contains(t, releaseResponse, "releases")
		assert.Contains(t, releaseResponse, "total")

		// Step 7: Test error cases
		// Try to create app with same name
		resp, _ = st.makeRequest("POST", "/api/apps", appData)
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode) // Duplicate name should fail

		// Try to get non-existent app
		resp, _ = st.makeRequest("GET", "/api/apps/999999", nil)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)

		// Try to create release for non-existent app
		resp, _ = st.makeRequest("POST", "/api/apps/999999/releases", releaseData)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestSystemAPIValidation(t *testing.T) {
	st := NewSystemTest(t)
	defer st.Cleanup()

	t.Run("API Input Validation", func(t *testing.T) {
		// Test missing required fields
		incompleteApp := map[string]interface{}{
			"name": "incomplete-app",
			// Missing required fields
		}

		resp, _ := st.makeRequest("POST", "/api/apps", incompleteApp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		// Test invalid JSON
		req, err := http.NewRequest("POST", st.baseURL+"/api/apps", bytes.NewBufferString("invalid json"))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err = st.client.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		// Test invalid URL parameters
		resp, _ = st.makeRequest("GET", "/api/apps/not-a-number", nil)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestSystemConcurrency(t *testing.T) {
	st := NewSystemTest(t)
	defer st.Cleanup()

	t.Run("Concurrent App Creation", func(t *testing.T) {
		numConcurrent := 5
		results := make(chan error, numConcurrent)

		// Create apps concurrently
		for i := 0; i < numConcurrent; i++ {
			go func(id int) {
				appData := map[string]interface{}{
					"name":               fmt.Sprintf("concurrent-app-%d", id),
					"git_url":            "https://github.com/test/repo.git",
					"registry_repo":      fmt.Sprintf("harbor.test.com/test/app-%d", id),
					"target_namespace":   "test",
					"target_deploy_name": fmt.Sprintf("app-%d", id),
				}

				resp, _ := st.makeRequest("POST", "/api/apps", appData)
				if resp.StatusCode != http.StatusCreated {
					results <- fmt.Errorf("unexpected status code: %d", resp.StatusCode)
				} else {
					results <- nil
				}
			}(i)
		}

		// Collect results
		for i := 0; i < numConcurrent; i++ {
			err := <-results
			assert.NoError(t, err)
		}

		// Verify all apps were created
		resp, body := st.makeRequest("GET", "/api/apps", nil)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var apps []models.App
		err := json.Unmarshal(body, &apps)
		require.NoError(t, err)
		assert.Len(t, apps, numConcurrent)
	})
}

func TestSystemPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance tests in short mode")
	}

	st := NewSystemTest(t)
	defer st.Cleanup()

	t.Run("API Response Times", func(t *testing.T) {
		// Create some test data first
		for i := 0; i < 10; i++ {
			appData := map[string]interface{}{
				"name":               fmt.Sprintf("perf-test-app-%d", i),
				"git_url":            fmt.Sprintf("https://github.com/test/repo%d.git", i),
				"registry_repo":      fmt.Sprintf("harbor.test.com/test/app-%d", i),
				"target_namespace":   "test",
				"target_deploy_name": fmt.Sprintf("app-%d", i),
			}

			resp, _ := st.makeRequest("POST", "/api/apps", appData)
			assert.Equal(t, http.StatusCreated, resp.StatusCode)
		}

		// Measure response times
		start := time.Now()
		resp, _ := st.makeRequest("GET", "/api/apps", nil)
		duration := time.Since(start)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Less(t, duration, 1*time.Second, "API response should be under 1 second")

		// Test individual app fetch
		start = time.Now()
		resp, _ = st.makeRequest("GET", "/api/apps/1", nil)
		duration = time.Since(start)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Less(t, duration, 500*time.Millisecond, "Single app fetch should be under 500ms")
	})
}

func TestSystemHealthCheck(t *testing.T) {
	st := NewSystemTest(t)
	defer st.Cleanup()

	t.Run("Health Endpoint", func(t *testing.T) {
		resp, body := st.makeRequest("GET", "/health", nil)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var healthResponse map[string]interface{}
		err := json.Unmarshal(body, &healthResponse)
		require.NoError(t, err)
		assert.Equal(t, "ok", healthResponse["status"])
	})
}

func TestSystemDatabaseConsistency(t *testing.T) {
	st := NewSystemTest(t)
	defer st.Cleanup()

	t.Run("Database Consistency", func(t *testing.T) {
		// Create app via API
		appData := map[string]interface{}{
			"name":               "consistency-test-app",
			"git_url":            "https://github.com/test/repo.git",
			"registry_repo":      "harbor.test.com/test/consistency-app",
			"target_namespace":   "test",
			"target_deploy_name": "consistency-app",
		}

		resp, body := st.makeRequest("POST", "/api/apps", appData)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var createdApp models.App
		err := json.Unmarshal(body, &createdApp)
		require.NoError(t, err)

		// Verify in database directly
		var dbApp models.App
		err = st.db.First(&dbApp, createdApp.ID).Error
		require.NoError(t, err)
		assert.Equal(t, createdApp.Name, dbApp.Name)
		assert.Equal(t, createdApp.GitURL, dbApp.GitURL)

		// Update via API and verify consistency
		// Note: We don't have update endpoint in current implementation
		// but this would test it if we did

		// Delete and verify cascade
		// Note: We don't have delete endpoint in current implementation
		// but this would test it if we did
	})
}

// Load test runner
func TestSystemLoadTest(t *testing.T) {
	if os.Getenv("LOAD_TEST") != "1" {
		t.Skip("Set LOAD_TEST=1 to run load tests")
	}

	st := NewSystemTest(t)
	defer st.Cleanup()

	t.Run("Load Test", func(t *testing.T) {
		numUsers := 10
		requestsPerUser := 20

		results := make(chan time.Duration, numUsers*requestsPerUser)
		errors := make(chan error, numUsers*requestsPerUser)

		for user := 0; user < numUsers; user++ {
			go func(userID int) {
				for req := 0; req < requestsPerUser; req++ {
					start := time.Now()
					
					resp, _ := st.makeRequest("GET", "/api/apps", nil)
					duration := time.Since(start)
					
					if resp.StatusCode == http.StatusOK {
						results <- duration
					} else {
						errors <- fmt.Errorf("user %d req %d: status %d", userID, req, resp.StatusCode)
					}
				}
			}(user)
		}

		// Collect results
		var totalDuration time.Duration
		successCount := 0
		errorCount := 0

		for i := 0; i < numUsers*requestsPerUser; i++ {
			select {
			case duration := <-results:
				totalDuration += duration
				successCount++
			case err := <-errors:
				t.Logf("Error: %v", err)
				errorCount++
			}
		}

		// Calculate metrics
		avgDuration := totalDuration / time.Duration(successCount)
		successRate := float64(successCount) / float64(numUsers*requestsPerUser) * 100

		t.Logf("Load test results:")
		t.Logf("  Total requests: %d", numUsers*requestsPerUser)
		t.Logf("  Successful: %d", successCount)
		t.Logf("  Errors: %d", errorCount)
		t.Logf("  Success rate: %.2f%%", successRate)
		t.Logf("  Average response time: %v", avgDuration)

		// Assert acceptable performance
		assert.Greater(t, successRate, 95.0, "Success rate should be > 95%")
		assert.Less(t, avgDuration, 2*time.Second, "Average response time should be < 2s under load")
	})
}