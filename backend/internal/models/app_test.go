package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAppModel(t *testing.T) {
	// Setup in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// Migrate schema
	err = db.AutoMigrate(&App{}, &Release{})
	assert.NoError(t, err)

	t.Run("Create App", func(t *testing.T) {
		app := App{
			Name:              "test-app",
			GitURL:            "https://github.com/test/repo.git",
			DefaultBranch:     "main",
			DockerfilePath:    "Dockerfile",
			ContextPath:       ".",
			RegistryRepo:      "harbor.test.com/team/test-app",
			TargetNamespace:   "production",
			TargetDeployName:  "test-app",
			GitSecretRef:      "git-secret",
			RegistrySecretRef: "registry-secret",
		}

		err := db.Create(&app).Error
		assert.NoError(t, err)
		assert.NotZero(t, app.ID)
		assert.NotZero(t, app.CreatedAt)
	})

	t.Run("Create App with Defaults", func(t *testing.T) {
		app := App{
			Name:             "test-app-2",
			GitURL:           "https://github.com/test/repo2.git",
			RegistryRepo:     "harbor.test.com/team/test-app-2",
			TargetNamespace:  "staging",
			TargetDeployName: "test-app-2",
		}

		err := db.Create(&app).Error
		assert.NoError(t, err)
		assert.Equal(t, "main", app.DefaultBranch)
		assert.Equal(t, "Dockerfile", app.DockerfilePath)
		assert.Equal(t, ".", app.ContextPath)
	})

	t.Run("Unique App Name", func(t *testing.T) {
		app1 := App{
			Name:             "unique-app",
			GitURL:           "https://github.com/test/repo1.git",
			RegistryRepo:     "harbor.test.com/team/unique-app",
			TargetNamespace:  "production",
			TargetDeployName: "unique-app",
		}

		app2 := App{
			Name:             "unique-app", // Same name
			GitURL:           "https://github.com/test/repo2.git",
			RegistryRepo:     "harbor.test.com/team/unique-app-2",
			TargetNamespace:  "staging",
			TargetDeployName: "unique-app-2",
		}

		err := db.Create(&app1).Error
		assert.NoError(t, err)

		err = db.Create(&app2).Error
		assert.Error(t, err) // Should fail due to unique constraint
	})
}

func TestReleaseModel(t *testing.T) {
	// Setup in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// Migrate schema
	err = db.AutoMigrate(&App{}, &Release{})
	assert.NoError(t, err)

	// Create test app
	app := App{
		Name:             "test-app",
		GitURL:           "https://github.com/test/repo.git",
		RegistryRepo:     "harbor.test.com/team/test-app",
		TargetNamespace:  "production",
		TargetDeployName: "test-app",
	}
	err = db.Create(&app).Error
	assert.NoError(t, err)

	t.Run("Create Release", func(t *testing.T) {
		now := time.Now()
		release := Release{
			AppID:       app.ID,
			Branch:      "main",
			CommitSHA:   "abc123456",
			ImageTag:    "harbor.test.com/team/test-app:abc123",
			ImageDigest: "sha256:1234567890abcdef",
			Status:      StatusSuccess,
			StartedAt:   &now,
			FinishedAt:  &now,
			K8sRef:      "test-app-release-1",
		}

		err := db.Create(&release).Error
		assert.NoError(t, err)
		assert.NotZero(t, release.ID)
		assert.NotZero(t, release.CreatedAt)
	})

	t.Run("Release with Default Status", func(t *testing.T) {
		release := Release{
			AppID:     app.ID,
			Branch:    "develop",
			CommitSHA: "def789012",
			ImageTag:  "harbor.test.com/team/test-app:def789",
			K8sRef:    "test-app-release-2",
		}

		err := db.Create(&release).Error
		assert.NoError(t, err)
		assert.Equal(t, StatusPending, release.Status)
	})

	t.Run("Release Status Values", func(t *testing.T) {
		validStatuses := []ReleaseStatus{
			StatusPending,
			StatusRunning,
			StatusSuccess,
			StatusFailed,
			StatusCanceled,
		}

		for _, status := range validStatuses {
			release := Release{
				AppID:     app.ID,
				Branch:    "main",
				CommitSHA: "test123",
				ImageTag:  "test:latest",
				Status:    status,
				K8sRef:    "test-release",
			}

			err := db.Create(&release).Error
			assert.NoError(t, err)
			assert.Equal(t, status, release.Status)

			// Clean up
			db.Delete(&release)
		}
	})
}

func TestDockerfileValidation(t *testing.T) {
	t.Run("Valid Dockerfile", func(t *testing.T) {
		validation := DockerfileValidation{
			Valid: true,
			Path:  "Dockerfile",
		}

		assert.True(t, validation.Valid)
		assert.Equal(t, "Dockerfile", validation.Path)
		assert.Empty(t, validation.Error)
	})

	t.Run("Invalid Dockerfile", func(t *testing.T) {
		validation := DockerfileValidation{
			Valid: false,
			Path:  "Dockerfile",
			Error: "File not found",
		}

		assert.False(t, validation.Valid)
		assert.Equal(t, "Dockerfile", validation.Path)
		assert.Equal(t, "File not found", validation.Error)
	})
}

func TestBranch(t *testing.T) {
	t.Run("Branch Structure", func(t *testing.T) {
		branch := Branch{
			Name: "main",
			SHA:  "abc123456789",
		}

		assert.Equal(t, "main", branch.Name)
		assert.Equal(t, "abc123456789", branch.SHA)
	})
}