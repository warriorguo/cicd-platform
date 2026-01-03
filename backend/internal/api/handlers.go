package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/warriorguo/cicd-platform/backend/internal/git"
	"github.com/warriorguo/cicd-platform/backend/internal/k8s"
	"github.com/warriorguo/cicd-platform/backend/internal/models"
)

type Handler struct {
	db        *gorm.DB
	gitClient *git.Client
	k8sClient *k8s.Client
}

func NewHandler(db *gorm.DB, gitClient *git.Client, k8sClient *k8s.Client) *Handler {
	return &Handler{
		db:        db,
		gitClient: gitClient,
		k8sClient: k8sClient,
	}
}

func (h *Handler) CreateApp(c *gin.Context) {
	var req struct {
		Name              string            `json:"name" binding:"required"`
		GitURL            string            `json:"git_url" binding:"required"`
		DefaultBranch     string            `json:"default_branch"`
		BuildType         models.BuildType  `json:"build_type"`
		DockerfilePath    string            `json:"dockerfile_path"`
		ContextPath       string            `json:"context_path"`
		RegistryRepo      string            `json:"registry_repo" binding:"required"`
		TargetNamespace   string            `json:"target_namespace" binding:"required"`
		TargetDeployName  string            `json:"target_deploy_name" binding:"required"`
		Replicas          int               `json:"replicas"`
		GitSecretRef      string            `json:"git_secret_ref"`
		RegistrySecretRef string            `json:"registry_secret_ref"`
		EnvVars           models.EnvVars    `json:"env_vars"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.DefaultBranch == "" {
		req.DefaultBranch = "main"
	}
	if req.BuildType == "" {
		req.BuildType = models.BuildTypeDockerfile
	}
	if req.DockerfilePath == "" {
		if req.BuildType == models.BuildTypeDockerCompose {
			req.DockerfilePath = "docker-compose.yml"
		} else {
			req.DockerfilePath = "Dockerfile"
		}
	}
	if req.ContextPath == "" {
		req.ContextPath = "."
	}
	if req.Replicas <= 0 {
		req.Replicas = 1
	}

	app := models.App{
		Name:              req.Name,
		GitURL:            req.GitURL,
		DefaultBranch:     req.DefaultBranch,
		BuildType:         req.BuildType,
		DockerfilePath:    req.DockerfilePath,
		ContextPath:       req.ContextPath,
		RegistryRepo:      req.RegistryRepo,
		TargetNamespace:   req.TargetNamespace,
		TargetDeployName:  req.TargetDeployName,
		Replicas:          req.Replicas,
		GitSecretRef:      req.GitSecretRef,
		RegistrySecretRef: req.RegistrySecretRef,
		EnvVars:           req.EnvVars,
	}

	if err := h.db.Create(&app).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create app"})
		return
	}

	c.JSON(http.StatusCreated, app)
}

func (h *Handler) GetApp(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid app ID"})
		return
	}

	var app models.App
	if err := h.db.First(&app, uint(id)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "App not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get app"})
		return
	}

	c.JSON(http.StatusOK, app)
}

func (h *Handler) ListApps(c *gin.Context) {
	var apps []models.App
	if err := h.db.Find(&apps).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list apps"})
		return
	}

	c.JSON(http.StatusOK, apps)
}

func (h *Handler) GetAppBranches(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid app ID"})
		return
	}

	var app models.App
	if err := h.db.First(&app, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "App not found"})
		return
	}

	repoInfo, err := git.ParseGitURL(app.GitURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Git URL"})
		return
	}

	token := c.GetHeader("X-Git-Token")
	branches, err := h.gitClient.ListBranches(c.Request.Context(), repoInfo, token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch branches"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"branches": branches})
}

func (h *Handler) ValidateDockerfile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid app ID"})
		return
	}

	var app models.App
	if err := h.db.First(&app, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "App not found"})
		return
	}

	branch := c.Query("branch")
	if branch == "" {
		branch = app.DefaultBranch
	}

	repoInfo, err := git.ParseGitURL(app.GitURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Git URL"})
		return
	}

	token := c.GetHeader("X-Git-Token")

	// Validate the build file (Dockerfile or docker-compose.yml)
	exists, err := h.gitClient.CheckDockerfile(c.Request.Context(), repoInfo, branch, app.DockerfilePath, token)
	if err != nil {
		fileType := "Dockerfile"
		if app.BuildType == models.BuildTypeDockerCompose {
			fileType = "docker-compose.yml"
		}
		c.JSON(http.StatusOK, models.DockerfileValidation{
			Valid: false,
			Path:  app.DockerfilePath,
			Error: fmt.Sprintf("%s validation failed: %v", fileType, err),
		})
		return
	}

	c.JSON(http.StatusOK, models.DockerfileValidation{
		Valid: exists,
		Path:  app.DockerfilePath,
	})
}

func (h *Handler) CreateRelease(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid app ID"})
		return
	}

	var req struct {
		Branch    string `json:"branch"`
		CommitSHA string `json:"commit_sha"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate required fields
	if req.CommitSHA == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "commit_sha is required"})
		return
	}
	if len(req.CommitSHA) < 7 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "commit_sha must be at least 7 characters long"})
		return
	}

	var app models.App
	if err := h.db.First(&app, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "App not found"})
		return
	}

	if req.Branch == "" {
		req.Branch = app.DefaultBranch
	}

	// Validate Dockerfile exists
	repoInfo, err := git.ParseGitURL(app.GitURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Git URL"})
		return
	}

	token := c.GetHeader("X-Git-Token")
	exists, err := h.gitClient.CheckDockerfile(c.Request.Context(), repoInfo, req.Branch, app.DockerfilePath, token)
	if err != nil || !exists {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Dockerfile not found at %s in branch %s", app.DockerfilePath, req.Branch),
		})
		return
	}

	// Check for running releases
	var runningCount int64
	h.db.Model(&models.Release{}).Where("app_id = ? AND status = ?", app.ID, models.StatusRunning).Count(&runningCount)
	if runningCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Another release is already running for this app"})
		return
	}

	// Generate image tag
	shortSHA := req.CommitSHA
	if len(shortSHA) > 7 {
		shortSHA = shortSHA[:7]
	}
	fullImageTag := fmt.Sprintf("%s:%s", app.RegistryRepo, shortSHA)

	// Create release record
	release := models.Release{
		AppID:     app.ID,
		Branch:    req.Branch,
		CommitSHA: req.CommitSHA,
		ImageTag:  fullImageTag,
		Status:    models.StatusPending,
	}

	if err := h.db.Create(&release).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create release"})
		return
	}

	// Create actual Tekton PipelineRun if k8s client is available
	if h.k8sClient != nil {
		pipelineReq := &k8s.PipelineRunRequest{
			AppName:           app.Name,
			GitURL:            app.GitURL,
			Branch:            req.Branch,
			CommitSHA:         req.CommitSHA,
			BuildType:         string(app.BuildType),
			DockerfilePath:    app.DockerfilePath,
			ContextPath:       app.ContextPath,
			ImageRepo:         app.RegistryRepo,
			ImageTag:          shortSHA,
			TargetNamespace:   app.TargetNamespace,
			TargetDeployName:  app.TargetDeployName,
			Replicas:          app.Replicas,
			GitSecretRef:      app.GitSecretRef,
			RegistrySecretRef: app.RegistrySecretRef,
			EnvVars:           app.EnvVars,
		}

		pipelineRunName, err := h.k8sClient.CreatePipelineRun(c.Request.Context(), pipelineReq)
		if err != nil {
			// If PipelineRun creation fails, mark release as failed
			release.Status = models.StatusFailed
			h.db.Save(&release)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create PipelineRun: %v", err)})
			return
		}

		release.K8sRef = pipelineRunName
		h.db.Save(&release)

		// Start a goroutine to watch PipelineRun status
		go h.watchPipelineRun(release.ID, pipelineRunName)
	} else {
		// Fallback: simulate pipeline execution for development
		pipelineRunName := fmt.Sprintf("%s-release-%d", app.Name, release.ID)
		release.K8sRef = pipelineRunName
		h.db.Save(&release)

		go func() {
			time.Sleep(2 * time.Second)
			now := time.Now()
			release.Status = models.StatusRunning
			release.StartedAt = &now
			h.db.Save(&release)

			time.Sleep(30 * time.Second)
			finishedAt := time.Now()
			release.Status = models.StatusSuccess
			release.FinishedAt = &finishedAt
			release.ImageDigest = "sha256:" + uuid.New().String()
			h.db.Save(&release)
		}()
	}

	c.JSON(http.StatusCreated, release)
}

// watchPipelineRun watches a PipelineRun and updates the release status
func (h *Handler) watchPipelineRun(releaseID uint, pipelineRunName string) {
	ctx := context.Background()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeout := time.After(2 * time.Hour) // Max 2 hours for a build

	for {
		select {
		case <-timeout:
			// Timeout - mark as failed
			var release models.Release
			if err := h.db.First(&release, releaseID).Error; err == nil {
				release.Status = models.StatusFailed
				now := time.Now()
				release.FinishedAt = &now
				h.db.Save(&release)
			}
			return

		case <-ticker.C:
			status, err := h.k8sClient.GetPipelineRunStatus(ctx, pipelineRunName)
			if err != nil {
				continue
			}

			var release models.Release
			if err := h.db.First(&release, releaseID).Error; err != nil {
				return
			}

			now := time.Now()

			switch status {
			case "Running":
				if release.Status != models.StatusRunning {
					release.Status = models.StatusRunning
					release.StartedAt = &now
					h.db.Save(&release)
				}

			case "Success":
				release.Status = models.StatusSuccess
				release.FinishedAt = &now
				// TODO: Get actual image digest from PipelineRun
				release.ImageDigest = "sha256:" + uuid.New().String()
				h.db.Save(&release)
				return

			case "Failed":
				release.Status = models.StatusFailed
				release.FinishedAt = &now
				h.db.Save(&release)
				return
			}
		}
	}
}

func (h *Handler) GetRelease(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid release ID"})
		return
	}

	var release models.Release
	if err := h.db.Preload("App").First(&release, uint(id)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Release not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get release"})
		return
	}

	c.JSON(http.StatusOK, release)
}

func (h *Handler) ListReleases(c *gin.Context) {
	appID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid app ID"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	var releases []models.Release
	var total int64

	h.db.Model(&models.Release{}).Where("app_id = ?", uint(appID)).Count(&total)
	
	if err := h.db.Where("app_id = ?", uint(appID)).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&releases).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list releases"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"releases": releases,
		"total":    total,
		"page":     page,
		"limit":    limit,
	})
}

func (h *Handler) RollbackRelease(c *gin.Context) {
	releaseID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid release ID"})
		return
	}

	var targetRelease models.Release
	if err := h.db.Preload("App").First(&targetRelease, uint(releaseID)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Release not found"})
		return
	}

	if targetRelease.Status != models.StatusSuccess {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Can only rollback to successful releases"})
		return
	}

	// Update deployment
	imageTag := targetRelease.ImageTag
	if targetRelease.ImageDigest != "" {
		imageTag = targetRelease.App.RegistryRepo + "@" + targetRelease.ImageDigest
	}

	ctx := context.Background()
	err = h.k8sClient.UpdateDeployment(ctx, targetRelease.App.TargetNamespace, targetRelease.App.TargetDeployName, imageTag)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update deployment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Rollback initiated",
		"release": targetRelease,
	})
}