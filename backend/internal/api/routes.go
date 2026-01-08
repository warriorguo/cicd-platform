package api

import (
	"github.com/gin-gonic/gin"
	"github.com/gin-contrib/cors"
)

func SetupRoutes(handler *Handler) *gin.Engine {
	r := gin.Default()

	// CORS middleware
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://localhost:8080"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Git-Token"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	api := r.Group("/api")
	{
		// Apps
		api.POST("/apps", handler.CreateApp)
		api.GET("/apps", handler.ListApps)
		api.GET("/apps/:id", handler.GetApp)
		api.GET("/apps/:id/branches", handler.GetAppBranches)
		api.GET("/apps/:id/deployments", handler.GetAppDeployments)
		api.POST("/apps/:id/scale", handler.ScaleDeployment)
		api.GET("/apps/:id/pods/:podName/logs", handler.GetPodLogs)
		api.GET("/apps/:id/pods/:podName/tty", handler.GetPodTTY)
		api.GET("/apps/:id/pods/:podName/describe", handler.GetPodDescribe)
		api.POST("/apps/:id/validate", handler.ValidateDockerfile)
		api.DELETE("/apps/:id", handler.DeleteApp)
		api.DELETE("/apps/:id/deployments/:releaseId", handler.DeleteDeployment)

		// Releases
		api.POST("/apps/:id/releases", handler.CreateRelease)
		api.GET("/apps/:id/releases", handler.ListReleases)
		api.GET("/releases/:id", handler.GetRelease)
		api.POST("/releases/:id/deploy", handler.DeployRelease)
		api.POST("/releases/:id/rollback", handler.RollbackRelease)
		
		// Build monitoring endpoints
		api.GET("/releases/:id/build-status", handler.GetBuildStatus)
		api.GET("/releases/:id/build-stages", handler.GetBuildStages)
		api.GET("/releases/:id/build-logs/:stage", handler.GetBuildLogsForStage)
		api.GET("/releases/:id/build-logs", handler.GetBuildLogs)
		
		// Streaming endpoints
		api.GET("/releases/:id/stream", handler.StreamRelease)      // WebSocket
		api.GET("/releases/:id/stream-sse", handler.StreamReleaseSSE) // SSE
	}

	return r
}