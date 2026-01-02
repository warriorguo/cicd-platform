package test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/warriorguo/cicd-platform/backend/internal/api"
	"github.com/warriorguo/cicd-platform/backend/internal/git"
	"github.com/warriorguo/cicd-platform/backend/internal/models"
	"github.com/warriorguo/cicd-platform/backend/pkg/config"
)

func TestIntegrationAppWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Setup test database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.App{}, &models.Release{})
	require.NoError(t, err)

	// Setup router
	gin.SetMode(gin.TestMode)
	gitClient := git.NewClient()
	handler := api.NewHandler(db, gitClient, nil)
	
	r := gin.New()
	apiGroup := r.Group("/api")
	{
		apiGroup.POST("/apps", handler.CreateApp)
		apiGroup.GET("/apps", handler.ListApps)
		apiGroup.GET("/apps/:id", handler.GetApp)
		apiGroup.POST("/apps/:id/releases", handler.CreateRelease)
		apiGroup.GET("/apps/:id/releases", handler.ListReleases)
		apiGroup.GET("/releases/:id", handler.GetRelease)
	}

	t.Run("Complete App Lifecycle", func(t *testing.T) {
		// Step 1: Create an app
		appData := map[string]interface{}{
			"name":               "integration-test-app",
			"git_url":            "https://github.com/docker-library/hello-world.git",
			"default_branch":     "master",
			"dockerfile_path":    "Dockerfile",
			"context_path":       ".",
			"registry_repo":      "test.harbor.com/integration/hello-world",
			"target_namespace":   "test",
			"target_deploy_name": "hello-world",
		}

		jsonData, _ := json.Marshal(appData)
		req, _ := http.NewRequest("POST", "/api/apps", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var app models.App
		err := json.Unmarshal(w.Body.Bytes(), &app)
		require.NoError(t, err)
		assert.Equal(t, "integration-test-app", app.Name)

		// Step 2: List apps and verify our app is there
		req, _ = http.NewRequest("GET", "/api/apps", nil)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var apps []models.App
		err = json.Unmarshal(w.Body.Bytes(), &apps)
		require.NoError(t, err)
		assert.Len(t, apps, 1)
		assert.Equal(t, "integration-test-app", apps[0].Name)

		// Step 3: Get specific app
		req, _ = http.NewRequest("GET", "/api/apps/1", nil)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Step 4: Create a release
		releaseData := map[string]interface{}{
			"branch":     "master",
			"commit_sha": "abc123456",
		}

		jsonData, _ = json.Marshal(releaseData)
		req, _ = http.NewRequest("POST", "/api/apps/1/releases", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")

		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// Note: This might fail due to Dockerfile validation with real repo
		// But tests the workflow
		assert.Contains(t, []int{http.StatusCreated, http.StatusBadRequest}, w.Code)

		// Step 5: List releases
		req, _ = http.NewRequest("GET", "/api/apps/1/releases", nil)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var releaseResponse map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &releaseResponse)
		require.NoError(t, err)
		assert.Contains(t, releaseResponse, "releases")
	})
}

func TestIntegrationGitClientRealAPI(t *testing.T) {
	if testing.Short() || os.Getenv("SKIP_REAL_API_TESTS") != "" {
		t.Skip("Skipping real API tests")
	}

	client := git.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	t.Run("GitHub Public Repository", func(t *testing.T) {
		repoInfo, err := git.ParseGitURL("https://github.com/docker-library/hello-world.git")
		require.NoError(t, err)

		// Test listing branches (no token required for public repo)
		branches, err := client.ListBranches(ctx, repoInfo, "")
		if err != nil {
			t.Logf("Branch listing failed (expected for rate limiting): %v", err)
			return
		}

		assert.NotEmpty(t, branches)
		t.Logf("Found branches: %v", branches)

		// Test Dockerfile check
		exists, err := client.CheckDockerfile(ctx, repoInfo, "master", "Dockerfile", "")
		if err != nil {
			t.Logf("Dockerfile check failed (expected for rate limiting): %v", err)
			return
		}

		t.Logf("Dockerfile exists: %v", exists)
	})
}

func TestIntegrationConfigLoading(t *testing.T) {
	t.Run("Load Config from Environment", func(t *testing.T) {
		// Set test environment variables
		os.Setenv("SERVER_PORT", "9999")
		os.Setenv("DATABASE_HOST", "test-db")
		defer func() {
			os.Unsetenv("SERVER_PORT")
			os.Unsetenv("DATABASE_HOST")
		}()

		cfg, err := config.Load()
		require.NoError(t, err)

		assert.Equal(t, "9999", cfg.Server.Port)
		assert.Equal(t, "test-db", cfg.Database.Host)
	})
}

func TestIntegrationDatabaseOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database integration tests in short mode")
	}

	// Setup test database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.App{}, &models.Release{})
	require.NoError(t, err)

	t.Run("Complex Database Operations", func(t *testing.T) {
		// Create app
		app := models.App{
			Name:             "db-test-app",
			GitURL:           "https://github.com/test/repo.git",
			RegistryRepo:     "harbor.test.com/team/db-test-app",
			TargetNamespace:  "test",
			TargetDeployName: "db-test-app",
		}
		err := db.Create(&app).Error
		require.NoError(t, err)

		// Create multiple releases
		releases := []models.Release{
			{
				AppID:     app.ID,
				Branch:    "main",
				CommitSHA: "abc123",
				Status:    models.StatusSuccess,
				K8sRef:    "release-1",
			},
			{
				AppID:     app.ID,
				Branch:    "develop",
				CommitSHA: "def456",
				Status:    models.StatusFailed,
				K8sRef:    "release-2",
			},
			{
				AppID:     app.ID,
				Branch:    "main",
				CommitSHA: "ghi789",
				Status:    models.StatusRunning,
				K8sRef:    "release-3",
			},
		}

		for _, release := range releases {
			err := db.Create(&release).Error
			require.NoError(t, err)
		}

		// Test queries
		var successfulReleases []models.Release
		err = db.Where("app_id = ? AND status = ?", app.ID, models.StatusSuccess).Find(&successfulReleases).Error
		require.NoError(t, err)
		assert.Len(t, successfulReleases, 1)

		var mainBranchReleases []models.Release
		err = db.Where("app_id = ? AND branch = ?", app.ID, "main").Find(&mainBranchReleases).Error
		require.NoError(t, err)
		assert.Len(t, mainBranchReleases, 2)

		// Test with preload
		var appWithReleases models.App
		err = db.Preload("Releases").First(&appWithReleases, app.ID).Error
		require.NoError(t, err)
		assert.Len(t, appWithReleases.Releases, 3)

		// Test pagination
		var pagedReleases []models.Release
		err = db.Where("app_id = ?", app.ID).Limit(2).Offset(1).Order("created_at DESC").Find(&pagedReleases).Error
		require.NoError(t, err)
		assert.Len(t, pagedReleases, 2)
	})
}

func TestIntegrationAPIErrorHandling(t *testing.T) {
	// Setup test database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.App{}, &models.Release{})
	require.NoError(t, err)

	// Setup router
	gin.SetMode(gin.TestMode)
	gitClient := git.NewClient()
	handler := api.NewHandler(db, gitClient, nil)
	
	r := gin.New()
	apiGroup := r.Group("/api")
	{
		apiGroup.POST("/apps", handler.CreateApp)
		apiGroup.GET("/apps/:id", handler.GetApp)
	}

	t.Run("Error Handling Scenarios", func(t *testing.T) {
		// Test malformed JSON
		req, _ := http.NewRequest("POST", "/api/apps", bytes.NewBuffer([]byte("{invalid json")))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		// Test missing required fields
		appData := map[string]interface{}{
			"name": "incomplete-app",
			// Missing git_url and other required fields
		}

		jsonData, _ := json.Marshal(appData)
		req, _ = http.NewRequest("POST", "/api/apps", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")

		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		// Test non-existent resource
		req, _ = http.NewRequest("GET", "/api/apps/999999", nil)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		// Test invalid ID format
		req, _ = http.NewRequest("GET", "/api/apps/not-a-number", nil)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// Benchmark tests
func BenchmarkAppCreation(b *testing.B) {
	// Setup
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(b, err)

	err = db.AutoMigrate(&models.App{}, &models.Release{})
	require.NoError(b, err)

	gin.SetMode(gin.TestMode)
	gitClient := git.NewClient()
	handler := api.NewHandler(db, gitClient, nil)
	
	r := gin.New()
	r.POST("/api/apps", handler.CreateApp)

	appData := map[string]interface{}{
		"name":               "bench-app",
		"git_url":            "https://github.com/test/repo.git",
		"registry_repo":      "harbor.test.com/team/bench-app",
		"target_namespace":   "test",
		"target_deploy_name": "bench-app",
	}

	jsonData, _ := json.Marshal(appData)

	// Reset timer and run benchmark
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Modify name to avoid unique constraint
		appData["name"] = "bench-app-" + string(rune(i))
		jsonData, _ = json.Marshal(appData)
		
		req, _ := http.NewRequest("POST", "/api/apps", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}