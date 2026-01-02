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
		api.POST("/apps/:id/validate", handler.ValidateDockerfile)

		// Releases
		api.POST("/apps/:id/releases", handler.CreateRelease)
		api.GET("/apps/:id/releases", handler.ListReleases)
		api.GET("/releases/:id", handler.GetRelease)
		api.POST("/releases/:id/rollback", handler.RollbackRelease)
		
		// Streaming endpoints
		api.GET("/releases/:id/stream", handler.StreamRelease)      // WebSocket
		api.GET("/releases/:id/stream-sse", handler.StreamReleaseSSE) // SSE
	}

	return r
}