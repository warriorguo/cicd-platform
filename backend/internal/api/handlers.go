package api

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
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
		ServicePort    int              `json:"service_port"`
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
	if req.ServicePort == 0 {
		req.ServicePort = 8080 // Default service port
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
		ServiceName:       req.Name,      // Default service name same as app name
		ServicePort:       req.ServicePort, // Use user-provided service port
		IngressEnabled:    true,          // Enable Ingress by default
		IngressHost:       "",            // Empty for path-based routing
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
			if err := h.db.Preload("App").First(&release, releaseID).Error; err != nil {
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
		EnvVars        []models.EnvVar `json:"env_vars"`
		MaxUnavailable int             `json:"maxUnavailable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
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

	// Save env vars to the app if provided
	if len(req.EnvVars) > 0 {
		release.App.EnvVars = req.EnvVars
		if err := h.db.Save(&release.App).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save environment variables"})
			return
		}
	}

	// For upgrade operation, determine replicas based on existing deployment
	replicas := 1 // Default for new deployment
	var lastDeployed models.Release
	if err := h.db.Where("app_id = ? AND status = ?", release.AppID, models.StatusDeployed).
		Order("created_at DESC").
		First(&lastDeployed).Error; err == nil {
		// If there's an existing deployment, get actual replica count from Kubernetes
		if h.k8sClient != nil {
			if desired, _, err := h.k8sClient.GetDeploymentReplicas(c.Request.Context(), release.App.TargetNamespace, release.App.TargetDeployName); err == nil {
				replicas = int(desired)
			}
		}
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
		// Set default maxUnavailable if not provided
		maxUnavailable := req.MaxUnavailable
		if maxUnavailable == 0 {
			maxUnavailable = 25 // Default to 25%
		}

		// Use request env vars if provided, otherwise use app's saved env vars
		envVars := req.EnvVars
		if len(envVars) == 0 {
			envVars = release.App.EnvVars
		}

		deployReq := &k8s.DeployPipelineRunRequest{
			AppName:          release.App.Name,
			ImageRepo:        release.App.RegistryRepo,
			ImageTag:         release.ImageTag[strings.LastIndex(release.ImageTag, ":")+1:], // Extract tag from full image
			TargetNamespace:  release.App.TargetNamespace,
			TargetDeployName: release.App.TargetDeployName,
			Replicas:         replicas,
			EnvVars:          envVars,
			MaxUnavailable:   maxUnavailable,
			ServicePort:      release.App.ServicePort,    // Use app's configured service port
			ServiceAccount:   release.App.ServiceAccount, // Use app's configured service account
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
			if err := h.db.Preload("App").First(&release, releaseID).Error; err != nil {
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
				
				// Create or update Ingress if enabled
				if release.App.IngressEnabled {
					serviceName := release.App.ServiceName
					if serviceName == "" {
						// Default service name is same as deployment name
						serviceName = release.App.TargetDeployName
					}
					
					ingressConfig := &k8s.IngressConfig{
						Namespace:    release.App.TargetNamespace,
						AppName:      release.App.Name,
						ServiceName:  serviceName,
						ServicePort:  release.App.ServicePort,
						CommitSHA:    release.CommitSHA,
						Host:         release.App.IngressHost, // Custom host if specified
						Path:         h.config.Ingress.DefaultPath,
						HostTemplate: h.config.Ingress.HostTemplate,
					}
					
					if err := h.k8sClient.CreateOrUpdateIngress(ctx, ingressConfig); err != nil {
						// Log error but don't fail the deployment
						log.Printf("Failed to create/update Ingress for app %s: %v", release.App.Name, err)
					} else {
						// Update release with Ingress info
						release.IngressCreated = true
						release.IngressPath = h.config.Ingress.DefaultPath
						
						// Set the actual host used (either custom or generated from template)
						if release.App.IngressHost != "" {
							release.IngressHost = release.App.IngressHost
						} else {
							// Generate host from template
							release.IngressHost = strings.Replace(h.config.Ingress.HostTemplate, "*", release.App.Name, 1)
						}
						log.Printf("Successfully created/updated Ingress for app %s with host %s", release.App.Name, release.IngressHost)
					}
				}
				
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

	// Get the most recent deployed release for this app (only one should be active)
	var deployedRelease models.Release
	if err := h.db.Where("app_id = ? AND status = ?", appID, models.StatusDeployed).
		Order("created_at DESC").
		First(&deployedRelease).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// No deployed releases found
			c.JSON(200, gin.H{"deployments": []gin.H{}})
			return
		}
		c.JSON(500, gin.H{"error": "Failed to fetch deployments"})
		return
	}

	// Get Kubernetes deployment status for the most recent deployed release
	var deployments []gin.H
	deploymentInfo, err := h.getKubernetesDeploymentInfo(c.Request.Context(), &app, &deployedRelease)
	if err != nil {
		// If we can't get K8s info, still include the release info
		deployments = append(deployments, gin.H{
			"release_id":   deployedRelease.ID,
			"image_tag":    deployedRelease.ImageTag,
			"commit_sha":   deployedRelease.CommitSHA,
			"deployed_at":  deployedRelease.FinishedAt,
			"status":       "unknown",
			"replicas":     gin.H{"available": 0, "ready": 0, "desired": 1},
			"error":        err.Error(),
		})
	} else {
		deployments = append(deployments, deploymentInfo)
	}

	c.JSON(200, gin.H{"deployments": deployments})
}

// getKubernetesDeploymentInfo retrieves deployment information from Kubernetes
func (h *Handler) getKubernetesDeploymentInfo(ctx context.Context, app *models.App, release *models.Release) (gin.H, error) {
	var desiredReplicas int32 = 1
	var readyReplicas int32 = 0
	
	// Get actual replica count from Kubernetes if client is available
	if h.k8sClient != nil {
		desired, ready, err := h.k8sClient.GetDeploymentReplicas(ctx, app.TargetNamespace, app.TargetDeployName)
		if err != nil {
			// Log error but continue with default values
			fmt.Printf("Failed to get deployment replicas: %v\n", err)
		} else {
			desiredReplicas = desired
			readyReplicas = ready
		}
	}
	
	// Get individual pod details
	var pods []gin.H
	if h.k8sClient != nil {
		podDetails, err := h.k8sClient.GetDeploymentPods(ctx, app.TargetNamespace, app.TargetDeployName)
		if err != nil {
			fmt.Printf("Failed to get deployment pods: %v\n", err)
		} else {
			pods = podDetails
		}
	}
	
	deploymentInfo := gin.H{
		"release_id":   release.ID,
		"image_tag":    release.ImageTag,
		"commit_sha":   release.CommitSHA,
		"deployed_at":  release.FinishedAt,
		"status":       "running", // TODO: Get actual status from K8s
		"replicas": gin.H{
			"desired":   desiredReplicas,
			"available": readyReplicas,
			"ready":     readyReplicas,
		},
		"namespace":     app.TargetNamespace,
		"deployment_name": app.TargetDeployName,
		"pods":         pods,
	}

	return deploymentInfo, nil
}

// ScaleDeployment scales an existing deployment
func (h *Handler) ScaleDeployment(c *gin.Context) {
	appID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid app ID"})
		return
	}

	var req struct {
		Replicas int `json:"replicas" binding:"required,min=0,max=100"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var app models.App
	if err := h.db.First(&app, appID).Error; err != nil {
		c.JSON(404, gin.H{"error": "Application not found"})
		return
	}

	// Check if there's already a deployed release
	var lastDeployed models.Release
	if err := h.db.Where("app_id = ? AND status = ?", appID, models.StatusDeployed).
		Order("created_at DESC").
		First(&lastDeployed).Error; err != nil {
		c.JSON(400, gin.H{"error": "No deployed release found to scale"})
		return
	}

	// Perform the actual scaling operation
	if h.k8sClient != nil {
		fmt.Printf("DEBUG: Scaling deployment %s in namespace %s to %d replicas\n", app.TargetDeployName, app.TargetNamespace, req.Replicas)
		err := h.k8sClient.ScaleDeployment(c.Request.Context(), app.TargetNamespace, app.TargetDeployName, int32(req.Replicas))
		if err != nil {
			fmt.Printf("ERROR: Failed to scale deployment: %v\n", err)
			c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to scale deployment: %v", err)})
			return
		}
		fmt.Printf("DEBUG: Successfully scaled deployment\n")
	} else {
		fmt.Printf("ERROR: Kubernetes client not available\n")
		c.JSON(500, gin.H{"error": "Kubernetes client not available"})
		return
	}
	
	c.JSON(200, gin.H{
		"message": "Deployment scaled successfully",
		"app_id": appID,
		"replicas": req.Replicas,
		"release_id": lastDeployed.ID,
		"namespace": app.TargetNamespace,
		"deployment": app.TargetDeployName,
	})
}

// GetPodLogs gets logs for a specific pod
func (h *Handler) GetPodLogs(c *gin.Context) {
	appID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid app ID"})
		return
	}

	podName := c.Param("podName")
	if podName == "" {
		c.JSON(400, gin.H{"error": "Pod name is required"})
		return
	}

	// Get container name from query parameter, default to first container
	containerName := c.Query("container")
	
	// Get the application to determine namespace
	var app models.App
	if err := h.db.First(&app, appID).Error; err != nil {
		c.JSON(404, gin.H{"error": "Application not found"})
		return
	}

	if h.k8sClient == nil {
		c.JSON(500, gin.H{"error": "Kubernetes client not available"})
		return
	}

	// If no container specified, get the first container from the pod
	if containerName == "" {
		pods, err := h.k8sClient.GetDeploymentPods(c.Request.Context(), app.TargetNamespace, app.TargetDeployName)
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to get pods information"})
			return
		}
		
		// Find the specific pod and get its first container
		for _, pod := range pods {
			if podInfo, ok := pod["name"].(string); ok && podInfo == podName {
				// For deployment pods, we can assume the main container has the same name as the deployment
				containerName = app.TargetDeployName
				break
			}
		}
		
		// Fallback to deployment name if we couldn't find the pod
		if containerName == "" {
			containerName = app.TargetDeployName
		}
	}

	// Get pod logs from the correct namespace
	logs, err := h.k8sClient.GetPodLogsInNamespace(c.Request.Context(), app.TargetNamespace, podName, containerName)
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to get pod logs: %v", err)})
		return
	}

	// Check if download is requested
	if c.Query("download") == "true" {
		// Set headers for file download
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s-%s.log\"", podName, containerName))
		c.Header("Content-Type", "text/plain")
		c.String(200, logs)
		return
	}

	// Return logs as JSON
	c.JSON(200, gin.H{
		"pod_name": podName,
		"container_name": containerName,
		"namespace": app.TargetNamespace,
		"logs": logs,
	})
}


// GetPodTTY handles WebSocket connections for pod TTY
func (h *Handler) GetPodTTY(c *gin.Context) {
	appId := c.Param("id")
	podName := c.Param("podName")
	containerName := c.Query("container")

	// Get app to determine namespace
	var app models.App
	if err := h.db.First(&app, appId).Error; err != nil {
		c.JSON(404, gin.H{"error": "Application not found"})
		return
	}

	// Default to first container if not specified
	if containerName == "" {
		containerName = "main"
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to upgrade to WebSocket"})
		return
	}
	defer conn.Close()

	// Execute TTY in pod
	ctx := context.Background()
	err = h.k8sClient.ExecPodTTY(ctx, app.TargetNamespace, podName, containerName, conn)
	if err != nil {
		// Send error message to client before closing
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Error: %s\r\n", err.Error())))
		return
	}
}

// GetPodDescribe returns detailed pod information including events
func (h *Handler) GetPodDescribe(c *gin.Context) {
	appId := c.Param("id")
	podName := c.Param("podName")

	// Get app to determine namespace
	var app models.App
	if err := h.db.First(&app, appId).Error; err != nil {
		c.JSON(404, gin.H{"error": "Application not found"})
		return
	}

	// Get pod description
	ctx := context.Background()
	describe, err := h.k8sClient.GetPodDescribe(ctx, app.TargetNamespace, podName)
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to describe pod: %s", err.Error())})
		return
	}

	c.JSON(200, gin.H{
		"pod_name":  podName,
		"namespace": app.TargetNamespace,
		"describe":  describe,
	})
}

// DeleteApp deletes an application and all its associated resources
func (h *Handler) DeleteApp(c *gin.Context) {
	appID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid app ID"})
		return
	}

	// Get the app first
	var app models.App
	if err := h.db.First(&app, appID).Error; err != nil {
		c.JSON(404, gin.H{"error": "Application not found"})
		return
	}

	// Check if there are any deployed pods
	var deployedReleasesCount int64
	if err := h.db.Model(&models.Release{}).
		Where("app_id = ? AND status = ?", appID, models.StatusDeployed).
		Count(&deployedReleasesCount).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to check deployed releases"})
		return
	}

	if deployedReleasesCount > 0 {
		// Check actual pods in Kubernetes
		if h.k8sClient != nil {
			pods, err := h.k8sClient.GetDeploymentPods(c.Request.Context(), app.TargetNamespace, app.TargetDeployName)
			if err == nil && len(pods) > 0 {
				c.JSON(400, gin.H{"error": "Cannot delete application with running pods. Please scale down the deployment first."})
				return
			}
		}
	}

	// Get all releases for this app to delete Harbor images BEFORE soft-deleting them
	var releases []models.Release
	if err := h.db.Where("app_id = ?", appID).Find(&releases).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch releases"})
		return
	}

	// Delete Harbor images BEFORE deleting the releases records
	log.Printf("Harbor configuration: Enabled=%v, Endpoint=%s", h.config.Harbor.Enabled, h.config.Harbor.Endpoint)
	log.Printf("Found %d releases for deletion", len(releases))
	if h.config.Harbor.Enabled {
		log.Printf("Harbor is enabled, processing image deletions...")
		for _, release := range releases {
			log.Printf("Processing release ID=%d, ImageTag=%s", release.ID, release.ImageTag)
			if release.ImageTag != "" {
				// Extract repository and tag from image tag
				// Format: registry/library/app-name:tag
				log.Printf("Calling deleteHarborImage for app %s, imageTag %s", app.Name, release.ImageTag)
				if err := h.deleteHarborImage(&app, release.ImageTag); err != nil {
					// Log error but continue with deletion
					log.Printf("Failed to delete Harbor image %s: %v", release.ImageTag, err)
				}
			}
		}
	} else {
		log.Printf("Harbor is disabled, skipping image deletion")
	}

	// Delete build logs
	if err := h.db.Where("release_id IN (SELECT id FROM releases WHERE app_id = ?)", appID).
		Delete(&models.BuildLog{}).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to delete build logs"})
		return
	}

	// Delete releases (soft delete)
	if err := h.db.Where("app_id = ?", appID).Delete(&models.Release{}).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to delete releases"})
		return
	}

	// Delete Ingress if it exists
	if h.k8sClient != nil {
		if err := h.k8sClient.DeleteIngress(c.Request.Context(), app.TargetNamespace, app.Name); err != nil {
			// Log error but continue with deletion
			log.Printf("Failed to delete Ingress for app %s: %v", app.Name, err)
		} else {
			log.Printf("Successfully deleted Ingress for app %s", app.Name)
		}
	}
	
	// Delete the app
	if err := h.db.Delete(&app).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to delete application"})
		return
	}

	c.JSON(200, gin.H{"message": "Application deleted successfully"})
}

// deleteHarborImage deletes an image repository from Harbor
func (h *Handler) deleteHarborImage(app *models.App, imageTag string) error {
	// Extract repository name from app's registry repo
	// Format: registry/project/repository or registry/project/repository/sub-repo
	repository := ""
	if app.RegistryRepo != "" {
		// Extract repository name from registry_repo field
		parts := strings.Split(app.RegistryRepo, "/")
		if len(parts) >= 3 {
			// Join all parts after the project name to handle multi-level repositories
			repository = strings.Join(parts[2:], "/") // Skip registry and project parts
		} else {
			return fmt.Errorf("invalid registry_repo format: %s", app.RegistryRepo)
		}
	} else {
		// Fallback: parse from imageTag
		parts := strings.Split(imageTag, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid image tag format: %s", imageTag)
		}
		repoPath := parts[0]
		// Extract repository name
		pathParts := strings.Split(repoPath, "/")
		if len(pathParts) < 3 {
			return fmt.Errorf("invalid repository path: %s", repoPath)
		}
		// Join all parts after the project name
		repository = strings.Join(pathParts[2:], "/")
	}

	// Use the project from config
	project := h.config.Harbor.Project

	log.Printf("Attempting to delete Harbor repository: %s/%s", project, repository)

	// Build the Harbor API URL to delete the entire repository
	// For nested repositories, we need to URL-encode the repository name
	// Harbor requires double encoding for slashes in repository names
	// First encode the slashes, then encode the whole string
	doubleEncodedRepo := strings.ReplaceAll(repository, "/", "%2F")
	doubleEncodedRepo = url.QueryEscape(doubleEncodedRepo)

	// Use URL if it contains a protocol, otherwise construct with https:// + Endpoint
	var harborBaseURL string
	if strings.HasPrefix(h.config.Harbor.URL, "http://") || strings.HasPrefix(h.config.Harbor.URL, "https://") {
		harborBaseURL = h.config.Harbor.URL
	} else {
		harborBaseURL = fmt.Sprintf("https://%s", h.config.Harbor.Endpoint)
	}
	harborURL := fmt.Sprintf("%s/api/v2.0/projects/%s/repositories/%s",
		harborBaseURL, project, doubleEncodedRepo)

	// Create HTTP client with basic auth
	req, err := http.NewRequest("DELETE", harborURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	req.SetBasicAuth(h.config.Harbor.Username, h.config.Harbor.Password)
	req.Header.Set("Accept", "application/json")

	// Skip TLS verification for local Harbor (not recommended for production)
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	log.Printf("Deleting Harbor repository: %s", harborURL)
	
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete Harbor repository: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("Harbor delete response status: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete Harbor repository, status: %d, body: %s", resp.StatusCode, string(body))
	}

	log.Printf("Successfully deleted Harbor repository: %s/%s", project, repository)
	return nil
}

// GetAppIngress gets the Ingress information for an app
func (h *Handler) GetAppIngress(c *gin.Context) {
	appID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid app ID"})
		return
	}

	// Get the app
	var app models.App
	if err := h.db.First(&app, appID).Error; err != nil {
		c.JSON(404, gin.H{"error": "Application not found"})
		return
	}

	// Get the latest deployed release
	var release models.Release
	if err := h.db.Where("app_id = ? AND status = ?", appID, models.StatusDeployed).
		Order("created_at DESC").
		First(&release).Error; err != nil {
		c.JSON(404, gin.H{"error": "No deployed release found"})
		return
	}

	// Get Ingress from Kubernetes if available
	ingressInfo := gin.H{
		"enabled": app.IngressEnabled,
		"created": release.IngressCreated,
		"path":    release.IngressPath,
		"host":    release.IngressHost,
	}

	if h.k8sClient != nil && release.IngressCreated {
		ingress, err := h.k8sClient.GetIngress(c.Request.Context(), app.TargetNamespace, app.Name)
		if err == nil && ingress != nil {
			// Extract the actual URL from the Ingress (host-based routing)
			if len(ingress.Spec.Rules) > 0 {
				rule := ingress.Spec.Rules[0]
				host := rule.Host
				
				if len(rule.HTTP.Paths) > 0 {
					path := rule.HTTP.Paths[0].Path
					// For host-based routing, URL is simply http://host + path
					ingressInfo["url"] = fmt.Sprintf("http://%s%s", host, path)
					ingressInfo["host"] = host
					ingressInfo["path"] = path
				}
			}

			// Add annotations
			ingressInfo["annotations"] = ingress.Annotations
		}
	}

	// If no Ingress found but should exist, generate expected URL
	if release.IngressCreated {
		if url, exists := ingressInfo["url"]; !exists || url == "" {
			expectedHost := release.IngressHost
			if expectedHost == "" {
				// Generate from template
				expectedHost = strings.Replace(h.config.Ingress.HostTemplate, "*", app.Name, 1)
			}
			expectedPath := release.IngressPath
			if expectedPath == "" {
				expectedPath = h.config.Ingress.DefaultPath
			}
			ingressInfo["url"] = fmt.Sprintf("http://%s%s", expectedHost, expectedPath)
		}
	}

	c.JSON(200, ingressInfo)
}

// DeleteDeployment deletes a specific deployment and all its pods
func (h *Handler) DeleteDeployment(c *gin.Context) {
	appID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid app ID"})
		return
	}

	releaseID, err := strconv.Atoi(c.Param("releaseId"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid release ID"})
		return
	}

	// Get the app first
	var app models.App
	if err := h.db.First(&app, appID).Error; err != nil {
		c.JSON(404, gin.H{"error": "Application not found"})
		return
	}

	// Get the release
	var release models.Release
	if err := h.db.First(&release, releaseID).Error; err != nil {
		c.JSON(404, gin.H{"error": "Release not found"})
		return
	}

	// Verify the release belongs to this app
	if release.AppID != uint(appID) {
		c.JSON(400, gin.H{"error": "Release does not belong to this application"})
		return
	}

	// Check if this release is deployed
	if release.Status != models.StatusDeployed {
		c.JSON(400, gin.H{"error": "Release is not currently deployed"})
		return
	}

	// Delete the deployment from Kubernetes
	deploymentName := app.TargetDeployName
	if deploymentName == "" {
		deploymentName = app.Name
	}

	if h.k8sClient != nil {
		err = h.k8sClient.DeleteDeployment(c.Request.Context(), app.TargetNamespace, deploymentName)
		if err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to delete Kubernetes deployment: %v", err)})
			return
		}
	} else {
		c.JSON(500, gin.H{"error": "Kubernetes client not available"})
		return
	}

	// Update the release status to indicate it's no longer deployed
	release.Status = models.StatusSuccess // Change back to success status
	if err := h.db.Save(&release).Error; err != nil {
		// Log error but don't fail the operation since K8s deletion succeeded
		log.Printf("Warning: Failed to update release status after deployment deletion: %v", err)
	}

	c.JSON(200, gin.H{"message": "Deployment deleted successfully"})
}
