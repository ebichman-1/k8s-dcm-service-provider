package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dcm-project/k8s-service-provider/internal/config"
	"github.com/dcm-project/k8s-service-provider/internal/deployment/api"
	"github.com/dcm-project/k8s-service-provider/internal/deployment/services"
	"github.com/dcm-project/k8s-service-provider/internal/k8s"
	namespaceAPI "github.com/dcm-project/k8s-service-provider/internal/namespace/api"
	namespaceServices "github.com/dcm-project/k8s-service-provider/internal/namespace/services"
	"go.uber.org/zap"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()
	if err := cfg.Validate(); err != nil {
		fmt.Printf("Invalid configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger, err := initLogger(cfg.Log)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := logger.Sync(); err != nil {
			// Log sync errors but don't exit - this is cleanup code
			fmt.Printf("Logger sync error during shutdown: %v\n", err)
		}
	}()

	logger.Info("Starting K8s Service Provider",
		zap.String("version", "1.0.0"),
		zap.Int("port", cfg.Server.Port),
	)

	// Initialize shared Kubernetes client
	k8sClient, err := k8s.NewClient(cfg.Kubernetes, logger)
	if err != nil {
		logger.Fatal("Failed to initialize Kubernetes client", zap.Error(err))
	}

	// Initialize deployment service
	deployService := services.NewDeploymentService(k8sClient, logger)

	// Initialize namespace service
	namespaceService := namespaceServices.NewNamespaceService(k8sClient, logger)

	// Setup HTTP routers
	deploymentRouter := api.SetupRouter(deployService, logger)
	namespaceHandler := namespaceAPI.NewHandler(namespaceService, logger)
	namespaceRouter := namespaceAPI.SetupRouter(namespaceHandler, logger)

	// Create HTTP servers
	deploymentServer := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      deploymentRouter,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	namespaceServer := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, 8081),
		Handler:      namespaceRouter,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	// Start deployment service in a goroutine
	go func() {
		logger.Info("Starting deployment service HTTP server", zap.String("address", deploymentServer.Addr))
		if err := deploymentServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start deployment server", zap.Error(err))
		}
	}()

	// Start namespace service in a goroutine
	go func() {
		logger.Info("Starting namespace service HTTP server", zap.String("address", namespaceServer.Addr))
		if err := namespaceServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start namespace server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal to gracefully shutdown both servers
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down servers...")

	// Give outstanding requests 30 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown both servers concurrently
	deploymentErr := make(chan error, 1)
	namespaceErr := make(chan error, 1)

	go func() {
		deploymentErr <- deploymentServer.Shutdown(ctx)
	}()

	go func() {
		namespaceErr <- namespaceServer.Shutdown(ctx)
	}()

	// Wait for both shutdowns to complete
	var shutdownErrors []error
	for i := 0; i < 2; i++ {
		select {
		case err := <-deploymentErr:
			if err != nil {
				logger.Error("Deployment server forced to shutdown", zap.Error(err))
				shutdownErrors = append(shutdownErrors, err)
			}
		case err := <-namespaceErr:
			if err != nil {
				logger.Error("Namespace server forced to shutdown", zap.Error(err))
				shutdownErrors = append(shutdownErrors, err)
			}
		}
	}

	if len(shutdownErrors) > 0 {
		os.Exit(1)
	}

	logger.Info("Both servers gracefully stopped")
}

// initLogger initializes the logger based on configuration
func initLogger(cfg config.LogConfig) (*zap.Logger, error) {
	var zapConfig zap.Config

	switch cfg.Level {
	case "debug":
		zapConfig = zap.NewDevelopmentConfig()
	case "info", "warn", "error":
		zapConfig = zap.NewProductionConfig()
	default:
		zapConfig = zap.NewProductionConfig()
	}

	// Set log level
	switch cfg.Level {
	case "debug":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		zapConfig.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	// Set output format
	if cfg.Format == "console" {
		zapConfig.Encoding = "console"
		zapConfig.EncoderConfig = zap.NewDevelopmentEncoderConfig()
	} else {
		zapConfig.Encoding = "json"
		zapConfig.EncoderConfig = zap.NewProductionEncoderConfig()
	}

	// Set output path
	if cfg.OutputPath != "" && cfg.OutputPath != "stdout" {
		zapConfig.OutputPaths = []string{cfg.OutputPath}
	}

	return zapConfig.Build()
}
