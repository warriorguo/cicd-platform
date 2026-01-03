package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
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

	// Mutex for WebSocket writes
	var wsMutex sync.Mutex

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
	
	wsMutex.Lock()
	if err := conn.WriteJSON(statusMsg); err != nil {
		wsMutex.Unlock()
		log.Printf("Error sending initial status: %v", err)
		return
	}
	wsMutex.Unlock()

	// Start status monitoring
	statusTicker := time.NewTicker(5 * time.Second)
	defer statusTicker.Stop()

	// Track active log streams
	activeStreams := make(map[string]bool)
	logStreamCtx, logStreamCancel := context.WithCancel(ctx)
	defer logStreamCancel()

	for {
		select {
		case <-ctx.Done():
			return

		case <-statusTicker.C:
			// Check for status updates
			var updatedRelease models.Release
			if err := h.db.Preload("App").First(&updatedRelease, release.ID).Error; err != nil {
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
				wsMutex.Lock()
				err := conn.WriteJSON(statusMsg)
				wsMutex.Unlock()
				
				if err != nil {
					log.Printf("Error sending status update: %v", err)
					return
				}

				// If completed, stop streaming
				if release.Status == models.StatusSuccess || release.Status == models.StatusFailed {
					return
				}
			}

			// Stream logs for running releases
			if release.Status == models.StatusRunning && release.K8sRef != "" && h.k8sClient != nil {
				// Get TaskRuns for this PipelineRun
				taskRuns, err := h.k8sClient.GetTaskRunsForPipelineRun(ctx, release.K8sRef)
				if err != nil {
					log.Printf("Error getting TaskRuns: %v", err)
					continue
				}

				// Start log streaming for each new pod
				for _, taskRun := range taskRuns {
					streamKey := fmt.Sprintf("%s-%s", taskRun.PodName, taskRun.TaskName)
					if !activeStreams[streamKey] {
						activeStreams[streamKey] = true
						
						// Start streaming logs for each container
						for _, container := range taskRun.Containers {
							go h.streamPodLogs(logStreamCtx, conn, &wsMutex, release.ID, taskRun.PodName, container, taskRun.TaskName)
						}
					}
				}
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

// streamPodLogs streams logs from a specific pod container
func (h *Handler) streamPodLogs(ctx context.Context, conn *websocket.Conn, wsMutex *sync.Mutex, releaseID uint, podName, containerName, taskName string) {
	logChan, errChan := h.k8sClient.StreamPodLogs(ctx, podName, containerName)
	
	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-errChan:
			if !ok {
				return
			}
			if err != nil {
				log.Printf("Error streaming logs from %s/%s: %v", podName, containerName, err)
				return
			}
		case logLine, ok := <-logChan:
			if !ok {
				return
			}
			if logLine != "" {
				logMsg := WSMessage{
					Type: "log",
					Payload: LogUpdate{
						ReleaseID: releaseID,
						Task:      taskName,
						Container: containerName,
						Content:   logLine,
						Timestamp: time.Now(),
					},
				}
				
				wsMutex.Lock()
				err := conn.WriteJSON(logMsg)
				wsMutex.Unlock()
				
				if err != nil {
					log.Printf("Error sending log: %v", err)
					return
				}
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