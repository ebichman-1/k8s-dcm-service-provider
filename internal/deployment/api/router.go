package api

import (
	"github.com/dcm-project/k8s-service-provider/internal/deployment/services"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SetupRouter sets up the HTTP router with all routes
func SetupRouter(deployService services.DeploymentServiceInterface, logger *zap.Logger) *gin.Engine {
	// Set Gin mode based on environment
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	// Add middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(CORSMiddleware())
	router.Use(LoggingMiddleware(logger))

	// Create handler
	handler := NewHandler(deployService, logger)

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Health check
		v1.GET("/health", handler.HealthCheck)

		// Deployment routes (AEP-compliant: PATCH for update, :deployment path param)
		deployments := v1.Group("/deployments")
		{
			deployments.POST("", handler.CreateDeployment)
			deployments.GET("", handler.ListDeployments)
			deployments.GET("/:deployment", handler.GetDeployment)
			deployments.PATCH("/:deployment", handler.UpdateDeployment)
			deployments.DELETE("/:deployment", handler.DeleteDeployment)
		}
	}

	return router
}

// CORSMiddleware adds CORS headers
func CORSMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PATCH, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})
}

// LoggingMiddleware adds structured logging to requests
func LoggingMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// Start timer
		start := c.Request.Context()

		// Process request
		c.Next()

		// Log request details
		logger.Info("HTTP request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("query", c.Request.URL.RawQuery),
			zap.String("ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.Int("status", c.Writer.Status()),
			zap.Int("size", c.Writer.Size()),
		)

		_ = start // Suppress unused variable warning
	})
}
