package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"

	"github.com/warriorguo/cicd-platform/backend/internal/models"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type ReleaseStatusUpdate struct {
	ReleaseID uint                  `json:"release_id"`
	Status    models.ReleaseStatus `json:"status"`
	StartedAt *time.Time           `json:"started_at,omitempty"`
	FinishedAt *time.Time          `json:"finished_at,omitempty"`
	ImageDigest string             `json:"image_digest,omitempty"`
}

type LogUpdate struct {
	ReleaseID uint   `json:"release_id"`
	Task      string `json:"task"`
	Container string `json:"container"`
	Content   string `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

func (h *Handler) StreamRelease(c *gin.Context) {
	releaseID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid release ID"})
		return
	}

	// Verify release exists
	var release models.Release
	if err := h.db.First(&release, uint(releaseID)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Release not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get release"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Send initial status
	statusMsg := WSMessage{
		Type: "status",
		Payload: ReleaseStatusUpdate{
			ReleaseID:   release.ID,
			Status:      release.Status,
			StartedAt:   release.StartedAt,
			FinishedAt:  release.FinishedAt,
			ImageDigest: release.ImageDigest,
		},
	}
	if err := conn.WriteJSON(statusMsg); err != nil {
		log.Printf("Error sending initial status: %v", err)
		return
	}

	// Start status monitoring
	statusTicker := time.NewTicker(5 * time.Second)
	defer statusTicker.Stop()

	// Start log streaming (simulated for now)
	logTicker := time.NewTicker(2 * time.Second)
	defer logTicker.Stop()

	logMessages := []string{
		"Starting git-clone task...",
		"Cloning repository...",
		"Repository cloned successfully",
		"Starting validate-dockerfile task...",
		"Dockerfile found at path: Dockerfile",
		"Dockerfile validation passed",
		"Starting kaniko-build task...",
		"Building container image...",
		"Pushing image to registry...",
		"Image pushed successfully",
		"Starting kubectl-deploy task...",
		"Updating deployment...",
		"Deployment updated successfully",
		"Starting verify-deployment task...",
		"Waiting for rollout to complete...",
		"Deployment rollout completed",
	}
	
	logIndex := 0
	var currentTask string

	for {
		select {
		case <-ctx.Done():
			return

		case <-statusTicker.C:
			// Check for status updates
			var updatedRelease models.Release
			if err := h.db.First(&updatedRelease, release.ID).Error; err != nil {
				continue
			}

			if updatedRelease.Status != release.Status ||
				updatedRelease.FinishedAt != release.FinishedAt ||
				updatedRelease.ImageDigest != release.ImageDigest {
				
				release = updatedRelease
				statusMsg := WSMessage{
					Type: "status",
					Payload: ReleaseStatusUpdate{
						ReleaseID:   release.ID,
						Status:      release.Status,
						StartedAt:   release.StartedAt,
						FinishedAt:  release.FinishedAt,
						ImageDigest: release.ImageDigest,
					},
				}
				if err := conn.WriteJSON(statusMsg); err != nil {
					log.Printf("Error sending status update: %v", err)
					return
				}

				// If completed, stop streaming
				if release.Status == models.StatusSuccess || release.Status == models.StatusFailed {
					return
				}
			}

		case <-logTicker.C:
			// Send simulated logs
			if release.Status == models.StatusRunning && logIndex < len(logMessages) {
				// Determine current task based on log message
				message := logMessages[logIndex]
				switch {
				case logIndex < 3:
					currentTask = "git-clone"
				case logIndex < 6:
					currentTask = "validate-dockerfile"
				case logIndex < 10:
					currentTask = "kaniko-build"
				case logIndex < 13:
					currentTask = "kubectl-deploy"
				default:
					currentTask = "verify-deployment"
				}

				logMsg := WSMessage{
					Type: "log",
					Payload: LogUpdate{
						ReleaseID: release.ID,
						Task:      currentTask,
						Container: "step",
						Content:   message,
						Timestamp: time.Now(),
					},
				}

				if err := conn.WriteJSON(logMsg); err != nil {
					log.Printf("Error sending log update: %v", err)
					return
				}
				logIndex++
			}

		default:
			// Handle client messages (ping/pong)
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			_, _, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("WebSocket error: %v", err)
				}
				return
			}
		}
	}
}

func (h *Handler) StreamReleaseSSE(c *gin.Context) {
	releaseID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid release ID"})
		return
	}

	// Verify release exists
	var release models.Release
	if err := h.db.First(&release, uint(releaseID)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Release not found"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// Send initial status
	statusData, _ := json.Marshal(ReleaseStatusUpdate{
		ReleaseID:   release.ID,
		Status:      release.Status,
		StartedAt:   release.StartedAt,
		FinishedAt:  release.FinishedAt,
		ImageDigest: release.ImageDigest,
	})
	c.SSEvent("status", string(statusData))
	c.Writer.Flush()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			// Check for updates
			var updatedRelease models.Release
			if err := h.db.First(&updatedRelease, release.ID).Error; err != nil {
				continue
			}

			if updatedRelease.Status != release.Status {
				release = updatedRelease
				statusData, _ := json.Marshal(ReleaseStatusUpdate{
					ReleaseID:   release.ID,
					Status:      release.Status,
					StartedAt:   release.StartedAt,
					FinishedAt:  release.FinishedAt,
					ImageDigest: release.ImageDigest,
				})
				c.SSEvent("status", string(statusData))
				c.Writer.Flush()

				if release.Status == models.StatusSuccess || release.Status == models.StatusFailed {
					return
				}
			}
		}
	}
}