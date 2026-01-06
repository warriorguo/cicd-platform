package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/warriorguo/cicd-platform/backend/internal/git"
	"github.com/warriorguo/cicd-platform/backend/internal/k8s"
	"github.com/warriorguo/cicd-platform/backend/internal/models"
	"github.com/warriorguo/cicd-platform/backend/pkg/config"
)

type Handler struct {
	db        *gorm.DB
	gitClient *git.Client
	k8sClient *k8s.Client
	config    *config.Config
}

func NewHandler(db *gorm.DB, gitClient *git.Client, k8sClient *k8s.Client, cfg *config.Config) *Handler {
	return &Handler{
		db:        db,
		gitClient: gitClient,
		k8sClient: k8sClient,
		config:    cfg,
	}
}

func (h *Handler) CreateApp(c *gin.Context) {
	var req struct {
		Name           string           `json:"name" binding:"required"`
		GitURL         string           `json:"git_url" binding:"required"`
		DefaultBranch  string           `json:"default_branch"`
		BuildType      models.BuildType `json:"build_type"`
		DockerfilePath string           `json:"dockerfile_path"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set defaults for optional fields
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

	// Generate default values from config
	registryRepo := fmt.Sprintf("%s/%s/%s", h.config.Harbor.Endpoint, h.config.Harbor.Project, req.Name)
	
	app := models.App{
		Name:              req.Name,
		GitURL:            req.GitURL,
		DefaultBranch:     req.DefaultBranch,
		BuildType:         req.BuildType,
		DockerfilePath:    req.DockerfilePath,
		ContextPath:       h.config.Defaults.ContextPath,
		RegistryRepo:      registryRepo,
		TargetNamespace:   h.config.Defaults.TargetNamespace,
		TargetDeployName:  req.Name, // Same as application name
		Replicas:          1,         // Default replicas, will be configurable in deploy stage
		GitSecretRef:      h.config.Defaults.GitSecretRef,
		RegistrySecretRef: h.config.Defaults.RegistrySecretRef,
		EnvVars:           models.EnvVars{}, // Empty by default, will be configurable in deploy stage
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

	// Create actual Tekton Build PipelineRun if k8s client is available
	fmt.Printf("DEBUG: k8sClient is nil: %v\n", h.k8sClient == nil)
	if h.k8sClient != nil {
		buildReq := &k8s.BuildPipelineRunRequest{
			AppName:           app.Name,
			GitURL:            app.GitURL,
			Branch:            req.Branch,
			CommitSHA:         req.CommitSHA,
			BuildType:         string(app.BuildType),
			DockerfilePath:    app.DockerfilePath,
			ContextPath:       app.ContextPath,
			ImageRepo:         app.RegistryRepo,
			ImageTag:          shortSHA,
			GitSecretRef:      app.GitSecretRef,
			RegistrySecretRef: app.RegistrySecretRef,
		}

		pipelineRunName, err := h.k8sClient.CreateBuildPipelineRun(c.Request.Context(), buildReq)
		if err != nil {
			// If PipelineRun creation fails, mark release as failed
			release.Status = models.StatusFailed
			h.db.Save(&release)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create Build PipelineRun: %v", err)})
			return
		}

		release.K8sRef = pipelineRunName
		h.db.Save(&release)

		// Start a goroutine to watch PipelineRun status
		fmt.Printf("DEBUG: Starting watchBuildPipelineRun for release %d, pipelineRunName: %s\n", release.ID, pipelineRunName)
		go h.watchBuildPipelineRun(release.ID, pipelineRunName)
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

// watchBuildPipelineRun watches a Build PipelineRun and updates the release status with detailed stage tracking
func (h *Handler) watchBuildPipelineRun(releaseID uint, pipelineRunName string) {
	fmt.Printf("DEBUG: watchBuildPipelineRun started for release %d, pipeline: %s\n", releaseID, pipelineRunName)
	ctx := context.Background()
	ticker := time.NewTicker(3 * time.Second) // Reduced from 5 to 3 seconds for faster updates
	defer ticker.Stop()

	timeout := time.After(2 * time.Hour) // Max 2 hours for a build
	trackedTaskRuns := make(map[string]models.BuildLogStatus) // TaskRun name -> last known status
	
	// Initial check to capture any TaskRuns that may have already completed
	fmt.Printf("Starting build monitoring for release %d, pipeline %s\n", releaseID, pipelineRunName)
	initialTaskRuns, err := h.k8sClient.GetTaskRunsForPipelineRun(ctx, pipelineRunName)
	if err == nil {
		for _, taskRun := range initialTaskRuns {
			status := h.mapTaskRunStatus(taskRun.Status)
			h.createOrUpdateBuildLog(releaseID, &taskRun, status)
			trackedTaskRuns[taskRun.TaskRunName] = status
			
			// If already completed, capture logs immediately
			if status == models.BuildLogStatusCompleted || status == models.BuildLogStatusFailed {
				fmt.Printf("TaskRun %s already completed with status %s, capturing logs\n", taskRun.TaskRunName, status)
				h.captureBuildLogs(releaseID, &taskRun)
			}
		}
	}

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
			// Get overall pipeline status
			pipelineStatus, err := h.k8sClient.GetPipelineRunStatus(ctx, pipelineRunName)
			if err != nil {
				continue
			}

			// Monitor running TaskRuns for stage tracking (for future use in real-time updates)
			_, err = h.k8sClient.GetRunningTaskRuns(ctx, pipelineRunName)
			if err != nil {
				continue
			}

			// Check for TaskRuns that have transitioned from running to completed/failed
			allTaskRuns, err := h.k8sClient.GetTaskRunsForPipelineRun(ctx, pipelineRunName)
			if err != nil {
				continue
			}

			// Process all TaskRuns to detect status changes and capture logs
			for _, taskRun := range allTaskRuns {
				lastKnownStatus, wasTracked := trackedTaskRuns[taskRun.TaskRunName]
				currentStatus := h.mapTaskRunStatus(taskRun.Status)

				// If TaskRun wasn't tracked before, initialize it
				if !wasTracked {
					h.createOrUpdateBuildLog(releaseID, &taskRun, currentStatus)
					trackedTaskRuns[taskRun.TaskRunName] = currentStatus
					continue
				}

				// Check for status transitions
				if lastKnownStatus != currentStatus {
					// Status changed, update build log
					h.createOrUpdateBuildLog(releaseID, &taskRun, currentStatus)
					trackedTaskRuns[taskRun.TaskRunName] = currentStatus

					// If TaskRun completed (succeeded or failed), capture logs
					if currentStatus == models.BuildLogStatusCompleted || currentStatus == models.BuildLogStatusFailed {
						h.captureBuildLogs(releaseID, &taskRun)
					}
				}
			}

			// Update overall release status
			var release models.Release
			if err := h.db.First(&release, releaseID).Error; err != nil {
				return
			}

			now := time.Now()

			switch pipelineStatus {
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
				
				// Final attempt to capture logs for all TaskRuns when pipeline fails
				allTaskRuns, err := h.k8sClient.GetTaskRunsForPipelineRun(ctx, pipelineRunName)
				if err == nil {
					for _, taskRun := range allTaskRuns {
						// Try to capture logs for any TaskRun we might have missed
						h.captureBuildLogs(releaseID, &taskRun)
					}
				}
				
				return
			}
		}
	}
}

// mapTaskRunStatus maps Kubernetes TaskRun status to BuildLogStatus
func (h *Handler) mapTaskRunStatus(status string) models.BuildLogStatus {
	fmt.Printf("DEBUG: TaskRun status mapping: '%s'\n", status)
	
	// Tekton TaskRun statuses are in format "True Succeeded", "False Failed", etc.
	if strings.Contains(status, "Succeeded") {
		return models.BuildLogStatusCompleted
	} else if strings.Contains(status, "Failed") {
		return models.BuildLogStatusFailed
	} else if strings.Contains(status, "Running") {
		return models.BuildLogStatusRunning
	} else {
		return models.BuildLogStatusPending
	}
}

// createOrUpdateBuildLog creates or updates a BuildLog entry for a TaskRun
func (h *Handler) createOrUpdateBuildLog(releaseID uint, taskRun *k8s.TaskRunInfo, status models.BuildLogStatus) {
	// Create individual entries for each container
	for _, containerName := range taskRun.Containers {
		var buildLog models.BuildLog

		// Try to find existing build log
		result := h.db.Where("release_id = ? AND task_run_name = ? AND container_name = ?", 
			releaseID, taskRun.TaskRunName, containerName).First(&buildLog)

		if result.Error != nil {
			// Create new build log
			buildLog = models.BuildLog{
				ReleaseID:     releaseID,
				StageName:     taskRun.TaskName,
				TaskRunName:   taskRun.TaskRunName,
				PodName:       taskRun.PodName,
				ContainerName: containerName,
				Status:        status,
				StartedAt:     taskRun.StartedAt,
				FinishedAt:    taskRun.FinishedAt,
			}
			h.db.Create(&buildLog)
		} else {
			// Update existing build log
			buildLog.Status = status
			if taskRun.StartedAt != nil {
				buildLog.StartedAt = taskRun.StartedAt
			}
			if taskRun.FinishedAt != nil {
				buildLog.FinishedAt = taskRun.FinishedAt
			}
			h.db.Save(&buildLog)
		}
	}
}

// captureBuildLogs captures the complete logs for a completed TaskRun
func (h *Handler) captureBuildLogs(releaseID uint, taskRun *k8s.TaskRunInfo) {
	ctx := context.Background()

	for _, containerName := range taskRun.Containers {
		// Find the build log record first
		var buildLog models.BuildLog
		result := h.db.Where("release_id = ? AND task_run_name = ? AND container_name = ?", 
			releaseID, taskRun.TaskRunName, containerName).First(&buildLog)

		if result.Error != nil {
			// BuildLog record doesn't exist - this shouldn't happen in normal flow
			continue
		}

		// Get the logs for this container
		logs, err := h.k8sClient.GetCompletedPodLogs(ctx, taskRun.PodName, containerName)
		
		// Update the build log with either logs or error message
		if err != nil {
			// Log error in the database so user can see what went wrong
			buildLog.ErrorMessage = fmt.Sprintf("Failed to retrieve logs: %v", err.Error())
			buildLog.LogsContent = "" // Ensure logs content is empty
		} else {
			buildLog.LogsContent = logs
			buildLog.ErrorMessage = "" // Clear any previous error
		}
		
		// Save the build log (with either logs or error message)
		h.db.Save(&buildLog)
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

func (h *Handler) DeployRelease(c *gin.Context) {
	releaseID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid release ID"})
		return
	}

	var req struct {
		Replicas int                 `json:"replicas"`
		EnvVars  []models.EnvVar     `json:"env_vars"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Set default replicas if not provided
	if req.Replicas <= 0 {
		req.Replicas = 1
	}

	var release models.Release
	if err := h.db.Preload("App").First(&release, uint(releaseID)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Release not found"})
		return
	}

	if release.Status != models.StatusSuccess {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Can only deploy successful builds"})
		return
	}

	// Check if there's already a deployment running for this app
	var runningDeployCount int64
	h.db.Model(&models.Release{}).Where("app_id = ? AND status = ? AND id != ?", release.AppID, models.StatusDeploying, release.ID).Count(&runningDeployCount)
	if runningDeployCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Another deployment is already running for this app"})
		return
	}

	// Update release status to deploying
	release.Status = models.StatusDeploying
	h.db.Save(&release)

	// Create actual Tekton Deploy PipelineRun if k8s client is available
	if h.k8sClient != nil {
		deployReq := &k8s.DeployPipelineRunRequest{
			AppName:          release.App.Name,
			ImageRepo:        release.App.RegistryRepo,
			ImageTag:         release.ImageTag[strings.LastIndex(release.ImageTag, ":")+1:], // Extract tag from full image
			TargetNamespace:  release.App.TargetNamespace,
			TargetDeployName: release.App.TargetDeployName,
			Replicas:         req.Replicas,
			EnvVars:          req.EnvVars,
		}

		pipelineRunName, err := h.k8sClient.CreateDeployPipelineRun(c.Request.Context(), deployReq)
		if err != nil {
			// If PipelineRun creation fails, mark release as failed
			release.Status = models.StatusFailed
			h.db.Save(&release)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create Deploy PipelineRun: %v", err)})
			return
		}

		// Store deploy pipeline run name separately or append to existing K8sRef
		release.K8sRef = fmt.Sprintf("%s,deploy:%s", release.K8sRef, pipelineRunName)
		h.db.Save(&release)

		// Start a goroutine to watch Deploy PipelineRun status
		go h.watchDeployPipelineRun(release.ID, pipelineRunName)
	} else {
		// Fallback: simulate deployment for development
		go func() {
			time.Sleep(10 * time.Second)
			now := time.Now()
			release.Status = models.StatusDeployed
			release.FinishedAt = &now
			h.db.Save(&release)
		}()
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Deployment initiated",
		"release": release,
	})
}

// watchDeployPipelineRun watches a Deploy PipelineRun and updates the release status
func (h *Handler) watchDeployPipelineRun(releaseID uint, pipelineRunName string) {
	ctx := context.Background()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeout := time.After(30 * time.Minute) // Max 30 minutes for a deployment

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
				if release.Status != models.StatusDeploying {
					release.Status = models.StatusDeploying
					h.db.Save(&release)
				}

			case "Success":
				release.Status = models.StatusDeployed
				release.FinishedAt = &now
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
func (h *Handler) GetBuildStatus(c *gin.Context) {
	releaseID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid release ID"})
		return
	}

	// Get the release
	var release models.Release
	if err := h.db.Preload("App").First(&release, uint(releaseID)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Release not found"})
		return
	}

	// Get build logs for this release
	var buildLogs []models.BuildLog
	h.db.Where("release_id = ?", releaseID).Order("created_at ASC").Find(&buildLogs)

	// Calculate progress
	totalStages := len(buildLogs)
	completedStages := 0
	currentStage := ""
	var runningStages []string

	for _, log := range buildLogs {
		if log.Status == models.BuildLogStatusCompleted {
			completedStages++
		} else if log.Status == models.BuildLogStatusRunning {
			runningStages = append(runningStages, log.StageName)
		}
	}

	if len(runningStages) > 0 {
		currentStage = runningStages[0] // Take the first running stage
	}

	// Calculate percentage (avoid division by zero)
	progressPercentage := 0
	if totalStages > 0 {
		progressPercentage = (completedStages * 100) / totalStages
	}

	response := gin.H{
		"release_id":          releaseID,
		"overall_status":      release.Status,
		"current_stage":       currentStage,
		"running_stages":      runningStages,
		"progress_percentage": progressPercentage,
		"completed_stages":    completedStages,
		"total_stages":        totalStages,
		"started_at":          release.StartedAt,
		"finished_at":         release.FinishedAt,
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) GetBuildStages(c *gin.Context) {
	releaseID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid release ID"})
		return
	}

	// Get build logs for this release
	var buildLogs []models.BuildLog
	if err := h.db.Where("release_id = ?", releaseID).Order("created_at ASC").Find(&buildLogs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch build stages"})
		return
	}

	// Group by stage name
	stageMap := make(map[string][]models.BuildLog)
	for _, log := range buildLogs {
		stageMap[log.StageName] = append(stageMap[log.StageName], log)
	}

	// Build response
	var stages []gin.H
	for stageName, logs := range stageMap {
		// Determine stage status (if any container failed, stage failed; if all completed, stage completed; etc.)
		stageStatus := models.BuildLogStatusPending
		var stageStartedAt, stageFinishedAt *time.Time
		hasRunning := false
		hasCompleted := false
		hasFailed := false

		for _, log := range logs {
			switch log.Status {
			case models.BuildLogStatusRunning:
				hasRunning = true
			case models.BuildLogStatusCompleted:
				hasCompleted = true
			case models.BuildLogStatusFailed:
				hasFailed = true
			}

			if log.StartedAt != nil && (stageStartedAt == nil || log.StartedAt.Before(*stageStartedAt)) {
				stageStartedAt = log.StartedAt
			}
			if log.FinishedAt != nil && (stageFinishedAt == nil || log.FinishedAt.After(*stageFinishedAt)) {
				stageFinishedAt = log.FinishedAt
			}
		}

		// Determine overall stage status
		if hasFailed {
			stageStatus = models.BuildLogStatusFailed
		} else if hasRunning {
			stageStatus = models.BuildLogStatusRunning
		} else if hasCompleted && len(logs) > 0 {
			// Check if all containers in this stage are completed
			allCompleted := true
			for _, log := range logs {
				if log.Status != models.BuildLogStatusCompleted {
					allCompleted = false
					break
				}
			}
			if allCompleted {
				stageStatus = models.BuildLogStatusCompleted
			}
		}

		stage := gin.H{
			"stage_name":   stageName,
			"status":       stageStatus,
			"started_at":   stageStartedAt,
			"finished_at":  stageFinishedAt,
			"containers":   len(logs),
		}

		stages = append(stages, stage)
	}

	c.JSON(http.StatusOK, gin.H{
		"release_id": releaseID,
		"stages":     stages,
	})
}

func (h *Handler) GetBuildLogsForStage(c *gin.Context) {
	releaseID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid release ID"})
		return
	}

	stageName := c.Param("stage")
	if stageName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Stage name is required"})
		return
	}

	// Get build logs for this stage
	var buildLogs []models.BuildLog
	if err := h.db.Where("release_id = ? AND stage_name = ?", releaseID, stageName).
		Order("created_at ASC").Find(&buildLogs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch build logs"})
		return
	}

	if len(buildLogs) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "No logs found for this stage"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"release_id":  releaseID,
		"stage_name":  stageName,
		"logs":        buildLogs,
	})
}

func (h *Handler) GetBuildLogs(c *gin.Context) {
	releaseID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid release ID"})
		return
	}

	// Optional query parameters
	stageName := c.Query("stage")
	containerName := c.Query("container")
	status := c.Query("status")

	// Build query
	query := h.db.Where("release_id = ?", releaseID)

	if stageName != "" {
		query = query.Where("stage_name = ?", stageName)
	}
	if containerName != "" {
		query = query.Where("container_name = ?", containerName)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	// Get build logs
	var buildLogs []models.BuildLog
	if err := query.Order("created_at ASC").Find(&buildLogs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch build logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"release_id": releaseID,
		"logs":       buildLogs,
		"count":      len(buildLogs),
	})
}

// GetAppDeployments gets the current deployment information for an application
func (h *Handler) GetAppDeployments(c *gin.Context) {
	appID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid app ID"})
		return
	}

	var app models.App
	if err := h.db.First(&app, appID).Error; err != nil {
		c.JSON(404, gin.H{"error": "Application not found"})
		return
	}

	// Get all deployed releases for this app
	var deployedReleases []models.Release
	if err := h.db.Where("app_id = ? AND status = ?", appID, models.StatusDeployed).
		Order("created_at DESC").
		Find(&deployedReleases).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch deployments"})
		return
	}

	// Get Kubernetes deployment status for each deployed release
	var deployments []gin.H
	for _, release := range deployedReleases {
		deploymentInfo, err := h.getKubernetesDeploymentInfo(c.Request.Context(), &app, &release)
		if err != nil {
			// If we can't get K8s info, still include the release info
			deployments = append(deployments, gin.H{
				"release_id":   release.ID,
				"image_tag":    release.ImageTag,
				"commit_sha":   release.CommitSHA,
				"deployed_at":  release.FinishedAt,
				"status":       "unknown",
				"replicas":     gin.H{"available": 0, "ready": 0, "desired": 1},
				"error":        err.Error(),
			})
			continue
		}
		deployments = append(deployments, deploymentInfo)
	}

	c.JSON(200, gin.H{"deployments": deployments})
}

// getKubernetesDeploymentInfo retrieves deployment information from Kubernetes
func (h *Handler) getKubernetesDeploymentInfo(ctx context.Context, app *models.App, release *models.Release) (gin.H, error) {
	// For now, return mock data since we need to implement K8s deployment status checking
	// This would normally query the Kubernetes API to get actual deployment status
	
	deploymentInfo := gin.H{
		"release_id":   release.ID,
		"image_tag":    release.ImageTag,
		"commit_sha":   release.CommitSHA,
		"deployed_at":  release.FinishedAt,
		"status":       "running", // This should come from K8s deployment status
		"replicas": gin.H{
			"desired":   1, // Should come from deployment spec
			"available": 1, // Should come from deployment status
			"ready":     1, // Should come from deployment status
		},
		"namespace":     app.TargetNamespace,
		"deployment_name": app.TargetDeployName,
	}

	return deploymentInfo, nil
}
