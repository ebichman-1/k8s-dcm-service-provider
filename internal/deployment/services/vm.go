package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/dcm-project/k8s-service-provider/internal/deployment/models"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
)

// VMService handles virtual machine deployment operations using KubeVirt
type VMService struct {
	k8sClient      kubernetes.Interface
	kubevirtClient kubecli.KubevirtClient
	logger         *zap.Logger
}

// NewVMService creates a new VM service instance
func NewVMService(k8sClient kubernetes.Interface, logger *zap.Logger) *VMService {
	// Create KubeVirt client using default config
	virtClient, err := kubecli.GetKubevirtClientFromClientConfig(kubecli.DefaultClientConfig(&pflag.FlagSet{}))
	if err != nil {
		logger.Fatal("Failed to create KubeVirt client", zap.Error(err))
	}

	return &VMService{
		k8sClient:      k8sClient,
		kubevirtClient: virtClient,
		logger:         logger,
	}
}

// CreateVM creates a new virtual machine deployment using KubeVirt
func (v *VMService) CreateVM(ctx context.Context, req *models.Deployment, id string) error {
	logger := v.logger.Named("vm_service").With(zap.String("deployment_id", id))
	logger.Info("Starting VM deployment")

	vmSpec, ok := req.Spec.(models.VMSpec)
	if !ok {
		return fmt.Errorf("invalid VM spec format")
	}

	namespace := req.Metadata.Namespace
	if namespace == "" {
		namespace = "default"
	}

	// Create namespace if it doesn't exist
	if err := v.ensureNamespace(ctx, namespace); err != nil {
		return fmt.Errorf("failed to ensure namespace: %w", err)
	}

	// Handle SSH key secret if needed
	sshSecretName, secretWasCreated, err := v.ensureSSHKeySecret(ctx, namespace, &vmSpec.VM, id)
	if err != nil {
		return fmt.Errorf("failed to ensure SSH key secret: %w", err)
	}

	// Create the VirtualMachine object
	memory := resource.MustParse(fmt.Sprintf("%dGi", vmSpec.VM.Ram))
	labels := models.BuildDeploymentLabels(id, req.Metadata.Name)
	// Store in VM labels if we created a secret with random name (for cleanup tracking)
	if secretWasCreated {
		labels[models.LabelSSHSecretCreated] = "true"
	}

	virtualMachine := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", req.Metadata.Name),
			Namespace:    namespace,
			Labels:       labels,
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			RunStrategy: &[]kubevirtv1.VirtualMachineRunStrategy{kubevirtv1.RunStrategyRerunOnFailure}[0],
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Architecture: "amd64",
					Domain: kubevirtv1.DomainSpec{
						CPU: &kubevirtv1.CPU{
							Cores: uint32(vmSpec.VM.Cpu), // #nosec G115
						},
						Memory: &kubevirtv1.Memory{
							Guest: &memory,
						},
						Devices: kubevirtv1.Devices{
							Disks: []kubevirtv1.Disk{
								{
									Name:      fmt.Sprintf("%s-disk", req.Metadata.Name),
									BootOrder: &[]uint{1}[0],
									DiskDevice: kubevirtv1.DiskDevice{
										Disk: &kubevirtv1.DiskTarget{
											Bus: kubevirtv1.DiskBusVirtio,
										},
									},
								},
								{
									Name:      "cloudinitdisk",
									BootOrder: &[]uint{2}[0],
									DiskDevice: kubevirtv1.DiskDevice{
										Disk: &kubevirtv1.DiskTarget{
											Bus: kubevirtv1.DiskBusVirtio,
										},
									},
								},
							},
							Interfaces: []kubevirtv1.Interface{
								{
									Name: "myvmnic",
									InterfaceBindingMethod: kubevirtv1.InterfaceBindingMethod{
										Bridge: &kubevirtv1.InterfaceBridge{},
									},
								},
							},
							Rng: &kubevirtv1.Rng{},
						},
						Features: &kubevirtv1.Features{
							ACPI: kubevirtv1.FeatureState{},
							SMM: &kubevirtv1.FeatureState{
								Enabled: &[]bool{true}[0],
							},
						},
						Machine: &kubevirtv1.Machine{
							Type: "pc-q35-rhel9.4.0",
						},
					},
					Networks: []kubevirtv1.Network{
						{
							Name: "myvmnic",
							NetworkSource: kubevirtv1.NetworkSource{
								Pod: &kubevirtv1.PodNetwork{},
							},
						},
					},
					TerminationGracePeriodSeconds: &[]int64{180}[0],
					Volumes: []kubevirtv1.Volume{
						{
							Name: fmt.Sprintf("%s-disk", req.Metadata.Name),
							VolumeSource: kubevirtv1.VolumeSource{
								ContainerDisk: &kubevirtv1.ContainerDiskSource{
									Image: v.getOSImage(vmSpec.VM.Os),
								},
							},
						},
						{
							Name: "cloudinitdisk",
							VolumeSource: kubevirtv1.VolumeSource{
								CloudInitNoCloud: &kubevirtv1.CloudInitNoCloudSource{
									UserData: v.generateCloudInitUserData(req.Metadata.Name, &vmSpec.VM),
								},
							},
						},
					},
				},
			},
		},
	}

	// Add SSH AccessCredentials if configured
	if sshSecretName != "" {
		virtualMachine.Spec.Template.Spec.AccessCredentials = []kubevirtv1.AccessCredential{
			{
				SSHPublicKey: &kubevirtv1.SSHPublicKeyAccessCredential{
					Source: kubevirtv1.SSHPublicKeyAccessCredentialSource{
						Secret: &kubevirtv1.AccessCredentialSecretSource{
							SecretName: sshSecretName,
						},
					},
					PropagationMethod: kubevirtv1.SSHPublicKeyAccessCredentialPropagationMethod{
						NoCloud: &kubevirtv1.NoCloudSSHPublicKeyAccessCredentialPropagation{},
					},
				},
			},
		}
	}

	// Create the VirtualMachine in the cluster
	_, err = v.kubevirtClient.VirtualMachine(namespace).Create(ctx, virtualMachine, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create VirtualMachine: %w", err)
	}

	logger.Info("Successfully created VM deployment")
	return nil
}

// GetVM retrieves VM deployment information
func (v *VMService) GetVM(ctx context.Context, id string) (*models.Deployment, error) {
	logger := v.logger.Named("vm_service").With(zap.String("deployment_id", id))

	// Search across all namespaces using label selector
	vms, err := v.kubevirtClient.VirtualMachine("").List(ctx, metav1.ListOptions{
		LabelSelector: models.BuildDeploymentSelector(id),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get virtual machine: %w", err)
	}

	if len(vms.Items) == 0 {
		return nil, models.NewErrDeploymentNotFound(id)
	}

	vm := vms.Items[0]

	response := &models.Deployment{
		Path: models.BuildResourcePath(id),
		ID:   id,
		Kind: models.DeploymentKindVM,
		Metadata: models.Metadata{
			Name:      vm.Name,
			Namespace: vm.Namespace,
			Labels:    vm.Labels,
		},
		Status: models.DeploymentStatus{
			Phase: v.getVMPhase(&vm),
		},
		CreatedAt: vm.CreationTimestamp.Time,
		UpdatedAt: vm.CreationTimestamp.Time,
	}

	logger.Info("Successfully retrieved VM deployment")
	return response, nil
}

// UpdateVM updates an existing VM deployment
func (v *VMService) UpdateVM(ctx context.Context, req *models.Deployment, id string) error {
	logger := v.logger.Named("vm_service").With(zap.String("deployment_id", id))
	logger.Info("Updating VM deployment")

	namespace := req.Metadata.Namespace
	if namespace == "" {
		namespace = "default"
	}

	// For simplicity, we'll delete and recreate the VM
	if err := v.DeleteVM(ctx, id, namespace); err != nil {
		logger.Warn("Failed to delete existing VM during update", zap.Error(err))
	}

	return v.CreateVM(ctx, req, id)
}

// DeleteVM deletes a virtual machine deployment
func (v *VMService) DeleteVM(ctx context.Context, id, namespace string) error {
	logger := v.logger.Named("vm_service").With(zap.String("deployment_id", id))
	logger.Info("Deleting VM deployment")

	if namespace == "" {
		namespace = "default"
	}

	// First, check if this VM created a secret with a random name
	vms, err := v.kubevirtClient.VirtualMachine(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: models.BuildDeploymentSelector(id),
	})
	if err == nil && len(vms.Items) > 0 {
		vm := vms.Items[0]
		// Only delete secrets if we created them (indicated by label)
		if vm.Labels[models.LabelSSHSecretCreated] == "true" {
			err := v.k8sClient.CoreV1().Secrets(namespace).DeleteCollection(ctx,
				metav1.DeleteOptions{},
				metav1.ListOptions{
					LabelSelector: models.BuildDeploymentSelector(id),
				})
			if err != nil {
				logger.Warn("Failed to delete associated secrets", zap.Error(err))
			} else {
				logger.Info("Deleted auto-created SSH secrets")
			}
		}
	}

	// Delete VirtualMachines
	err = v.kubevirtClient.VirtualMachine(namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: models.BuildDeploymentSelector(id),
	})
	if err != nil {
		return fmt.Errorf("failed to delete VirtualMachine: %w", err)
	}

	logger.Info("Successfully deleted VM deployment")
	return nil
}

// ListVMs lists all VM deployments
func (v *VMService) ListVMs(ctx context.Context, namespace string) ([]models.Deployment, error) {
	logger := v.logger.Named("vm_service")

	vms, err := v.kubevirtClient.VirtualMachine(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: models.BuildManagedResourceSelector(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list virtual machines: %w", err)
	}

	var responses []models.Deployment
	for _, vm := range vms.Items {
		appID := vm.Labels[models.LabelAppID]

		response := models.Deployment{
			Path: models.BuildResourcePath(appID),
			ID:   appID,
			Kind: models.DeploymentKindVM,
			Metadata: models.Metadata{
				Name:      vm.Name,
				Namespace: vm.Namespace,
				Labels:    vm.Labels,
			},
			Status: models.DeploymentStatus{
				Phase: v.getVMPhase(&vm),
			},
			CreatedAt: vm.CreationTimestamp.Time,
			UpdatedAt: vm.CreationTimestamp.Time,
		}
		responses = append(responses, response)
	}

	logger.Info("Successfully listed VM deployments", zap.Int("count", len(responses)))
	return responses, nil
}

// generateRandomString generates a random hex string of specified length
func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length/2+1)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes)[:length], nil
}

// generateSecretName creates a randomized secret name for SSH keys
func (v *VMService) generateSecretName(deploymentName string) string {
	randomSuffix, err := generateRandomString(8)
	if err != nil {
		// Fallback to a timestamp-based suffix if random generation fails
		randomSuffix = fmt.Sprintf("%d", metav1.Now().Unix()%100000000)
	}
	return fmt.Sprintf("%s-ssh-key-%s", deploymentName, randomSuffix)
}

// validateSSHPublicKey validates that the provided string is a valid SSH public key
func (v *VMService) validateSSHPublicKey(publicKey string) error {
	publicKey = strings.TrimSpace(publicKey)

	if publicKey == "" {
		return fmt.Errorf("SSH public key cannot be empty")
	}

	// Valid SSH key types
	validPrefixes := []string{
		"ssh-rsa ",
		"ssh-ed25519 ",
		"ecdsa-sha2-nistp256 ",
		"ecdsa-sha2-nistp384 ",
		"ecdsa-sha2-nistp521 ",
		"ssh-dss ", // Legacy, but still valid
	}

	for _, prefix := range validPrefixes {
		if strings.HasPrefix(publicKey, prefix) {
			return nil
		}
	}

	return fmt.Errorf("invalid SSH public key format: must start with ssh-rsa, ssh-ed25519, ecdsa-sha2-*, or ssh-dss")
}

// validateSecretName validates Kubernetes secret name follows DNS-1123 subdomain rules
func (v *VMService) validateSecretName(name string) error {
	if name == "" {
		return fmt.Errorf("secret name cannot be empty")
	}

	// Kubernetes DNS-1123 subdomain: [a-z0-9]([-a-z0-9]*[a-z0-9])?
	matched, err := regexp.MatchString(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`, name)
	if err != nil {
		return fmt.Errorf("failed to validate secret name: %w", err)
	}

	if !matched {
		return fmt.Errorf("invalid secret name: must be lowercase alphanumeric with hyphens, start and end with alphanumeric")
	}

	if len(name) > 253 {
		return fmt.Errorf("secret name too long: maximum 253 characters")
	}

	return nil
}

// createSSHKeySecret creates a Kubernetes secret containing the SSH public key
func (v *VMService) createSSHKeySecret(ctx context.Context, namespace, secretName, publicKey, deploymentID string) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels:    models.BuildDeploymentLabels(deploymentID, secretName),
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"key": publicKey,
		},
	}

	_, err := v.k8sClient.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create SSH key secret: %w", err)
	}
	return nil
}

// ensureSSHKeySecret manages SSH key secret creation/validation
// Returns: (secretName, wasCreated bool, error)
func (v *VMService) ensureSSHKeySecret(ctx context.Context, namespace string, vmConfig *models.VMConfig, deploymentID string) (string, bool, error) {
	// Case 1: Neither field set - no SSH key
	if vmConfig.SshPublicKey == nil && vmConfig.SshKeyName == nil {
		return "", false, nil
	}

	// Validate inputs if provided
	if vmConfig.SshPublicKey != nil {
		if err := v.validateSSHPublicKey(*vmConfig.SshPublicKey); err != nil {
			return "", false, fmt.Errorf("invalid SSH public key: %w", err)
		}
	}

	// Determine secret name (user-provided or generated)
	var secretName string
	useRandomName := false
	if vmConfig.SshKeyName != nil {
		if err := v.validateSecretName(*vmConfig.SshKeyName); err != nil {
			return "", false, fmt.Errorf("invalid secret name: %w", err)
		}

		secretName = *vmConfig.SshKeyName
		_, err := v.k8sClient.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
		if err == nil {
			// Secret exists - use it (ignore ssh_public_key if set)
			return secretName, false, nil
		}
		if !errors.IsNotFound(err) {
			// Some other error occurred
			return "", false, fmt.Errorf("failed to check for secret %s: %w", secretName, err)
		}

		// Secret doesn't exist - need to create it
		if vmConfig.SshPublicKey == nil {
			// FAIL: ssh_key_name set but no public key and secret doesn't exist
			return "", false, fmt.Errorf("secret %s not found and no ssh_public_key provided", secretName)
		}
	} else {
		secretName = v.generateSecretName(fmt.Sprintf("vm-%s", deploymentID[:min(len(deploymentID), 8)]))
		useRandomName = true
	}

	// Create the secret (either random name or user-specified name that doesn't exist)
	if err := v.createSSHKeySecret(ctx, namespace, secretName, *vmConfig.SshPublicKey, deploymentID); err != nil {
		return "", false, err
	}

	// Return true for wasCreated only if we used a random name
	return secretName, useRandomName, nil
}

// ensureNamespace creates namespace if it doesn't exist
func (v *VMService) ensureNamespace(ctx context.Context, namespace string) error {
	_, err := v.k8sClient.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		_, err = v.k8sClient.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create namespace %s: %w", namespace, err)
		}
	}
	return nil
}

// getOSImage returns the container image for the specified OS
func (v *VMService) getOSImage(os string) string {
	images := map[string]string{
		"fedora": "quay.io/containerdisks/fedora:latest",
		"ubuntu": "quay.io/containerdisks/ubuntu:latest",
		"centos": "quay.io/containerdisks/centos:latest",
		"rhel":   "quay.io/containerdisks/rhel:latest",
	}

	if image, exists := images[os]; exists {
		return image
	}
	// Default to fedora if OS not found
	return "quay.io/containerdisks/fedora:latest"
}

// generateCloudInitUserData generates cloud-init user data for the VM
func (v *VMService) generateCloudInitUserData(appName string, vm *models.VMConfig) string {
	return fmt.Sprintf(`#cloud-config
user: %s
password: auto-generated-pass
chpasswd: { expire: False }
hostname: %s
`, vm.Os, appName)
}

// getVMPhase converts KubeVirt VM status to our deployment phase
func (v *VMService) getVMPhase(vm *kubevirtv1.VirtualMachine) models.DeploymentPhase {
	if vm.Status.Ready {
		return models.DeploymentPhaseRunning
	}

	for _, condition := range vm.Status.Conditions {
		if condition.Type == kubevirtv1.VirtualMachineReady {
			if condition.Status == corev1.ConditionTrue {
				return models.DeploymentPhaseRunning
			}
		}
		if condition.Type == kubevirtv1.VirtualMachineFailure {
			if condition.Status == corev1.ConditionTrue {
				return models.DeploymentPhaseFailed
			}
		}
	}

	return models.DeploymentPhasePending
}
