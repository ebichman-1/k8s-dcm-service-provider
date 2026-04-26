package services

import (
	"context"
	"fmt"

	"github.com/dcm-project/k8s-service-provider/internal/deployment/models"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
)

// ContainerService handles container deployment operations
type ContainerService struct {
	client kubernetes.Interface
	logger *zap.Logger
}

// NewContainerService creates a new container service instance
func NewContainerService(client kubernetes.Interface, logger *zap.Logger) *ContainerService {
	return &ContainerService{
		client: client,
		logger: logger,
	}
}

// CreateContainer creates a new container deployment
func (c *ContainerService) CreateContainer(ctx context.Context, req *models.Deployment, id string) error {
	logger := c.logger.Named("container_service").With(zap.String("deployment_id", id))
	logger.Info("Starting container deployment")

	containerSpec, ok := req.Spec.(models.ContainerSpec)
	if !ok {
		return fmt.Errorf("invalid container spec format")
	}

	namespace := req.Metadata.Namespace
	if namespace == "" {
		namespace = "default"
	}

	// Create namespace if it doesn't exist
	if err := c.ensureNamespace(ctx, namespace); err != nil {
		return fmt.Errorf("failed to ensure namespace: %w", err)
	}

	// Create deployment
	if err := c.createDeployment(ctx, req.Metadata.Name, namespace, &containerSpec, req.Metadata.Labels, id); err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	// Create service if ports are specified
	if len(containerSpec.Container.Ports) > 0 {
		if err := c.createService(ctx, req.Metadata.Name, namespace, &containerSpec, req.Metadata.Labels, id); err != nil {
			return fmt.Errorf("failed to create service: %w", err)
		}
	}

	logger.Info("Successfully created container deployment")
	return nil
}

// GetContainer retrieves container deployment information searching across all namespaces
func (c *ContainerService) GetContainer(ctx context.Context, id string) (*models.Deployment, error) {
	logger := c.logger.Named("container_service").With(zap.String("deployment_id", id))

	// Search across all namespaces using label selector
	deployments, err := c.client.AppsV1().Deployments("").List(ctx, metav1.ListOptions{
		LabelSelector: models.BuildDeploymentSelector(id),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}

	if len(deployments.Items) == 0 {
		return nil, models.NewErrDeploymentNotFound(id)
	}

	deployment := deployments.Items[0]

	response := &models.Deployment{
		Path: models.BuildResourcePath(id),
		ID:   id,
		Kind: models.DeploymentKindContainer,
		Metadata: models.Metadata{
			Name:      deployment.Name,
			Namespace: deployment.Namespace,
			Labels:    deployment.Labels,
		},
		Status: models.DeploymentStatus{
			Phase:         c.getDeploymentPhase(&deployment),
			ReadyReplicas: int(deployment.Status.ReadyReplicas),
		},
		CreatedAt: deployment.CreationTimestamp.Time,
		UpdatedAt: deployment.CreationTimestamp.Time,
	}

	logger.Info("Successfully retrieved container deployment")
	return response, nil
}

// UpdateContainer updates an existing container deployment
func (c *ContainerService) UpdateContainer(ctx context.Context, req *models.Deployment, id string) error {
	logger := c.logger.Named("container_service").With(zap.String("deployment_id", id))
	logger.Info("Updating container deployment")

	namespace := req.Metadata.Namespace
	if namespace == "" {
		namespace = "default"
	}

	// For simplicity, we'll delete and recreate the deployment
	if err := c.DeleteContainer(ctx, id, namespace); err != nil {
		logger.Warn("Failed to delete existing deployment during update", zap.Error(err))
	}

	return c.CreateContainer(ctx, req, id)
}

// DeleteContainer deletes a container deployment
func (c *ContainerService) DeleteContainer(ctx context.Context, id, namespace string) error {
	logger := c.logger.Named("container_service").With(zap.String("deployment_id", id))
	logger.Info("Deleting container deployment")

	if namespace == "" {
		namespace = "default"
	}

	// Delete deployment
	err := c.client.AppsV1().Deployments(namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: models.BuildDeploymentSelector(id),
	})
	if err != nil {
		logger.Error("Failed to delete deployment", zap.Error(err))
		return fmt.Errorf("failed to delete deployment: %w", err)
	}

	// Delete services
	services, err := c.client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: models.BuildDeploymentSelector(id),
	})
	if err != nil {
		logger.Warn("Failed to list services for deletion", zap.Error(err))
	} else {
		for _, service := range services.Items {
			err = c.client.CoreV1().Services(namespace).Delete(ctx, service.Name, metav1.DeleteOptions{})
			if err != nil {
				logger.Warn("Failed to delete service", zap.String("service", service.Name), zap.Error(err))
			}
		}
	}

	logger.Info("Successfully deleted container deployment")
	return nil
}

// ListContainers lists all container deployments
func (c *ContainerService) ListContainers(ctx context.Context, namespace string) ([]models.Deployment, error) {
	logger := c.logger.Named("container_service")

	deployments, err := c.client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: models.BuildManagedResourceSelector(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}

	var responses []models.Deployment
	for _, deployment := range deployments.Items {
		appID := deployment.Labels[models.LabelAppID]
		response := models.Deployment{
			Path: models.BuildResourcePath(appID),
			ID:   appID,
			Kind: models.DeploymentKindContainer,
			Metadata: models.Metadata{
				Name:      deployment.Name,
				Namespace: deployment.Namespace,
				Labels:    deployment.Labels,
			},
			Status: models.DeploymentStatus{
				Phase:         c.getDeploymentPhase(&deployment),
				ReadyReplicas: int(deployment.Status.ReadyReplicas),
			},
			CreatedAt: deployment.CreationTimestamp.Time,
			UpdatedAt: deployment.CreationTimestamp.Time,
		}
		responses = append(responses, response)
	}

	logger.Info("Successfully listed container deployments", zap.Int("count", len(responses)))
	return responses, nil
}

// ensureNamespace creates namespace if it doesn't exist
func (c *ContainerService) ensureNamespace(ctx context.Context, namespace string) error {
	_, err := c.client.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		_, err = c.client.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create namespace %s: %w", namespace, err)
		}
	}
	return nil
}

// createDeployment creates a Kubernetes deployment
func (c *ContainerService) createDeployment(ctx context.Context, name, namespace string, spec *models.ContainerSpec, labels map[string]string, id string) error {
	if labels == nil {
		labels = make(map[string]string)
	}
	// Merge user labels with deployment labels
	deploymentLabels := models.BuildDeploymentLabels(id, name)
	for k, v := range deploymentLabels {
		labels[k] = v
	}

	replicas := int32(ptr.Deref(spec.Container.Replicas, 1)) // #nosec G115

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   fmt.Sprintf("%s-%s", name, id[:8]),
			Labels: labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: models.BuildDeploymentLabels(id, name),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: models.BuildDeploymentLabels(id, name),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  name,
							Image: spec.Container.Image,
						},
					},
				},
			},
		},
	}

	// Add ports if specified
	if len(spec.Container.Ports) > 0 {
		var containerPorts []corev1.ContainerPort
		for _, port := range spec.Container.Ports {
			containerPorts = append(containerPorts, corev1.ContainerPort{
				ContainerPort: int32(port.ContainerPort), // #nosec G115
				Protocol:      corev1.ProtocolTCP,
			})
		}
		deployment.Spec.Template.Spec.Containers[0].Ports = containerPorts
	}

	// Add resources if specified
	if spec.Container.Resources != nil {
		resources := corev1.ResourceRequirements{
			Requests: corev1.ResourceList{},
		}

		if spec.Container.Resources.CPU != "" {
			if cpu, err := resource.ParseQuantity(spec.Container.Resources.CPU); err == nil {
				resources.Requests[corev1.ResourceCPU] = cpu
			}
		}

		if spec.Container.Resources.Memory != "" {
			if memory, err := resource.ParseQuantity(spec.Container.Resources.Memory); err == nil {
				resources.Requests[corev1.ResourceMemory] = memory
			}
		}

		deployment.Spec.Template.Spec.Containers[0].Resources = resources
	}

	// Add environment variables if specified
	if len(spec.Container.Environment) > 0 {
		var envVars []corev1.EnvVar
		for _, envVar := range spec.Container.Environment {
			envVars = append(envVars, corev1.EnvVar{
				Name:  envVar.Name,
				Value: envVar.Value,
			})
		}
		deployment.Spec.Template.Spec.Containers[0].Env = envVars
	}

	_, err := c.client.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
	return err
}

// createService creates a Kubernetes service
func (c *ContainerService) createService(ctx context.Context, name, namespace string, spec *models.ContainerSpec, labels map[string]string, id string) error {
	if labels == nil {
		labels = make(map[string]string)
	}
	// Merge user labels with deployment labels
	deploymentLabels := models.BuildDeploymentLabels(id, name)
	for k, v := range deploymentLabels {
		labels[k] = v
	}

	var servicePorts []corev1.ServicePort
	for _, port := range spec.Container.Ports {
		servicePort := int32(port.ContainerPort) // #nosec G115
		if port.ServicePort > 0 {
			servicePort = int32(port.ServicePort) // #nosec G115
		}

		servicePorts = append(servicePorts, corev1.ServicePort{
			Port:       servicePort,
			TargetPort: intstr.FromInt(port.ContainerPort),
			Protocol:   corev1.ProtocolTCP,
		})
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   fmt.Sprintf("%s-service-%s", name, id[:8]),
			Labels: labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: models.BuildDeploymentLabels(id, name),
			Ports:    servicePorts,
			Type:     corev1.ServiceTypeNodePort,
		},
	}

	_, err := c.client.CoreV1().Services(namespace).Create(ctx, service, metav1.CreateOptions{})
	return err
}

// getDeploymentPhase determines the deployment phase from Kubernetes deployment status
func (c *ContainerService) getDeploymentPhase(deployment *appsv1.Deployment) models.DeploymentPhase {
	if deployment.Status.ReadyReplicas == 0 {
		return models.DeploymentPhasePending
	}
	if deployment.Status.ReadyReplicas == *deployment.Spec.Replicas {
		return models.DeploymentPhaseRunning
	}
	return models.DeploymentPhasePending
}
