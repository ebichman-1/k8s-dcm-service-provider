package services

import (
	"context"
	"fmt"

	"github.com/dcm-project/k8s-service-provider/internal/deployment/models"
	"github.com/dcm-project/k8s-service-provider/internal/k8s"
	"go.uber.org/zap"
)

// DeploymentServiceInterface defines the interface for deployment operations
type DeploymentServiceInterface interface {
	CreateDeployment(ctx context.Context, req *models.Deployment, id string) error
	GetDeploymentByID(ctx context.Context, id string) (*models.Deployment, error)
	UpdateDeployment(ctx context.Context, req *models.Deployment, id string) error
	DeleteDeployment(ctx context.Context, id string) error
	ListDeployments(ctx context.Context, req *models.ListDeploymentsRequest) (*models.ListDeploymentsResponse, error)
}

// DeploymentService orchestrates container and VM deployments
type DeploymentService struct {
	containerService *ContainerService
	vmService        *VMService
	logger           *zap.Logger
}

// NewDeploymentService creates a new deployment service
func NewDeploymentService(k8sClient k8s.ClientInterface, logger *zap.Logger) *DeploymentService {
	return &DeploymentService{
		containerService: NewContainerService(k8sClient.GetClientset(), logger),
		vmService:        NewVMService(k8sClient.GetClientset(), logger),
		logger:           logger,
	}
}

// CreateDeployment creates a new deployment based on the kind
func (d *DeploymentService) CreateDeployment(ctx context.Context, req *models.Deployment, id string) error {
	logger := d.logger.Named("deployment_service").With(
		zap.String("kind", string(req.Kind)),
		zap.String("name", req.Metadata.Name),
		zap.String("deployment_id", id),
	)

	logger.Info("Creating deployment")

	// Check for global ID uniqueness before creating
	existingDeployment, err := d.GetDeploymentByID(ctx, id)
	if err == nil {
		// Deployment with this ID already exists
		logger.Error("Deployment ID already exists",
			zap.String("deployment_id", id),
			zap.String("existing_namespace", existingDeployment.Metadata.Namespace),
			zap.String("existing_kind", string(existingDeployment.Kind)))
		return models.NewErrDeploymentAlreadyExists(id, existingDeployment.Metadata.Namespace, existingDeployment.Kind)
	}

	// If error is multiple deployments found, that's also a conflict
	if models.IsMultipleFoundError(err) {
		logger.Error("Multiple deployments with same ID already exist", zap.String("deployment_id", id))
		return err // Return the original multiple found error
	}

	// If error is "deployment not found", that's what we want - proceed with creation
	if !models.IsNotFoundError(err) {
		// Some other error occurred during lookup
		logger.Error("Failed to check deployment ID uniqueness", zap.Error(err))
		return fmt.Errorf("failed to validate deployment ID uniqueness: %w", err)
	}

	switch req.Kind {
	case models.DeploymentKindContainer:
		return d.containerService.CreateContainer(ctx, req, id)
	case models.DeploymentKindVM:
		return d.vmService.CreateVM(ctx, req, id)
	default:
		return fmt.Errorf("unsupported deployment kind: %s", req.Kind)
	}
}

// GetDeployment retrieves a deployment by ID and kind
func (d *DeploymentService) GetDeployment(ctx context.Context, id, namespace string, kind models.DeploymentKind) (*models.Deployment, error) {
	logger := d.logger.Named("deployment_service").With(
		zap.String("kind", string(kind)),
		zap.String("deployment_id", id),
	)

	logger.Info("Getting deployment")

	switch kind {
	case models.DeploymentKindContainer:
		return d.containerService.GetContainer(ctx, id)
	case models.DeploymentKindVM:
		return d.vmService.GetVM(ctx, id)
	default:
		return nil, fmt.Errorf("unsupported deployment kind: %s", kind)
	}
}

// UpdateDeployment updates an existing deployment
func (d *DeploymentService) UpdateDeployment(ctx context.Context, req *models.Deployment, id string) error {
	logger := d.logger.Named("deployment_service").With(
		zap.String("kind", string(req.Kind)),
		zap.String("name", req.Metadata.Name),
		zap.String("deployment_id", id),
	)

	logger.Info("Updating deployment")

	switch req.Kind {
	case models.DeploymentKindContainer:
		return d.containerService.UpdateContainer(ctx, req, id)
	case models.DeploymentKindVM:
		return d.vmService.UpdateVM(ctx, req, id)
	default:
		return fmt.Errorf("unsupported deployment kind: %s", req.Kind)
	}
}

// DeleteDeployment deletes a deployment by ID (auto-detects namespace and kind)
func (d *DeploymentService) DeleteDeployment(ctx context.Context, id string) error {
	logger := d.logger.Named("deployment_service").With(zap.String("deployment_id", id))

	logger.Info("Deleting deployment")

	// First, find the deployment to get its details
	deployment, err := d.GetDeploymentByID(ctx, id)
	if err != nil {
		return err // This will include "multiple deployments found" or "deployment not found" errors
	}

	// Delete based on the found deployment's kind
	switch deployment.Kind {
	case models.DeploymentKindContainer:
		return d.containerService.DeleteContainer(ctx, id, deployment.Metadata.Namespace)
	case models.DeploymentKindVM:
		return d.vmService.DeleteVM(ctx, id, deployment.Metadata.Namespace)
	default:
		return fmt.Errorf("unsupported deployment kind: %s", deployment.Kind)
	}
}

// ListDeployments lists deployments with filtering and token-based pagination (AEP-0132)
func (d *DeploymentService) ListDeployments(ctx context.Context, req *models.ListDeploymentsRequest) (*models.ListDeploymentsResponse, error) {
	logger := d.logger.Named("deployment_service").With(
		zap.String("namespace", req.Namespace),
		zap.String("kind", string(req.Kind)),
		zap.Int("max_page_size", req.MaxPageSize),
	)

	logger.Info("Listing deployments")

	var allDeployments []models.Deployment

	// List containers if kind is empty or container
	if req.Kind == "" || req.Kind == models.DeploymentKindContainer {
		containers, err := d.containerService.ListContainers(ctx, req.Namespace)
		if err != nil {
			logger.Error("Failed to list containers", zap.Error(err))
			return nil, fmt.Errorf("failed to list containers: %w", err)
		}
		allDeployments = append(allDeployments, containers...)
	}

	// List VMs if kind is empty or vm
	if req.Kind == "" || req.Kind == models.DeploymentKindVM {
		vms, err := d.vmService.ListVMs(ctx, req.Namespace)
		if err != nil {
			logger.Error("Failed to list VMs", zap.Error(err))
			return nil, fmt.Errorf("failed to list VMs: %w", err)
		}
		allDeployments = append(allDeployments, vms...)
	}

	// Decode page token to get offset
	offset, err := models.DecodePageToken(req.PageToken)
	if err != nil {
		return nil, fmt.Errorf("invalid page token: %w", err)
	}

	pageSize := req.MaxPageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	// Apply pagination
	total := len(allDeployments)
	start := offset
	end := start + pageSize

	var nextPageToken string
	if start >= total {
		allDeployments = []models.Deployment{}
	} else {
		if end > total {
			end = total
		} else if end < total {
			nextPageToken = models.EncodePageToken(end)
		}
		allDeployments = allDeployments[start:end]
	}

	response := &models.ListDeploymentsResponse{
		Results:       allDeployments,
		NextPageToken: nextPageToken,
	}

	logger.Info("Successfully listed deployments", zap.Int("count", len(allDeployments)))
	return response, nil
}

// GetDeploymentByID retrieves a deployment by ID, searching both containers and VMs across all namespaces
func (d *DeploymentService) GetDeploymentByID(ctx context.Context, id string) (*models.Deployment, error) {
	logger := d.logger.Named("deployment_service").With(zap.String("deployment_id", id))

	var foundDeployments []*models.Deployment

	// Try to find as container
	if deployment, err := d.containerService.GetContainer(ctx, id); err == nil {
		foundDeployments = append(foundDeployments, deployment)
	}

	// Try to find as VM
	if deployment, err := d.vmService.GetVM(ctx, id); err == nil {
		foundDeployments = append(foundDeployments, deployment)
	}

	// Check for conflicts (multiple deployments with same ID)
	if len(foundDeployments) > 1 {
		logger.Error("Multiple deployments found with same ID",
			zap.String("deployment_id", id),
			zap.Int("count", len(foundDeployments)))

		// Extract namespaces for better error context
		namespaces := make([]string, len(foundDeployments))
		for i, deployment := range foundDeployments {
			namespaces[i] = deployment.Metadata.Namespace
		}
		return nil, models.NewErrMultipleDeploymentsFound(id, len(foundDeployments), namespaces...)
	}

	// Return the found deployment
	if len(foundDeployments) == 1 {
		return foundDeployments[0], nil
	}

	logger.Warn("Deployment not found", zap.String("deployment_id", id))
	return nil, models.NewErrDeploymentNotFound(id)
}
