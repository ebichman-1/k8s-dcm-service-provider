package models

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"time"
)

// DeploymentKind represents the type of deployment
type DeploymentKind string

const (
	DeploymentKindContainer DeploymentKind = "container"
	DeploymentKindVM        DeploymentKind = "vm"
)

// Label keys for Kubernetes resources
const (
	LabelManagedBy        = "managed-by"
	LabelAppID            = "app-id"
	LabelApp              = "app"
	LabelSSHSecretCreated = "ssh-secret-created" // #nosec G101
)

// Label values
const (
	LabelValueManagedBy = "k8s-service-provider"
)

// Deployment represents the unified resource for creating, reading, updating deployments (AEP-compliant)
type Deployment struct {
	Path      string           `json:"path,omitempty"`
	ID        string           `json:"id,omitempty"`
	Kind      DeploymentKind   `json:"kind" binding:"required,oneof=container vm"`
	Metadata  Metadata         `json:"metadata" binding:"required"`
	Spec      interface{}      `json:"spec" binding:"required"`
	Status    DeploymentStatus `json:"status,omitempty"`
	CreatedAt time.Time        `json:"create_time,omitempty"`
	UpdatedAt time.Time        `json:"update_time,omitempty"`
}

// Metadata represents common metadata for deployments
type Metadata struct {
	Name      string            `json:"name" binding:"required,max=63,min=1"`
	Namespace string            `json:"namespace,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// ContainerSpec represents the specification for container deployments
type ContainerSpec struct {
	Container ContainerConfig `json:"container" binding:"required"`
}

// ContainerConfig represents container configuration
type ContainerConfig struct {
	Image       string                `json:"image" binding:"required"`
	Replicas    *int                  `json:"replicas,omitempty"`
	Ports       []PortConfig          `json:"ports,omitempty"`
	Resources   *ResourceConfig       `json:"resources,omitempty"`
	Environment []EnvironmentVariable `json:"environment,omitempty"`
}

// PortConfig represents port configuration
type PortConfig struct {
	ContainerPort int    `json:"container_port" binding:"required,min=1,max=65535"`
	ServicePort   int    `json:"service_port,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
}

// ResourceConfig represents resource configuration
type ResourceConfig struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// EnvironmentVariable represents an environment variable
type EnvironmentVariable struct {
	Name  string `json:"name" binding:"required"`
	Value string `json:"value" binding:"required"`
}

// VMSpec represents the specification for virtual machine deployments
type VMSpec struct {
	VM VMConfig `json:"vm" binding:"required"`
}

// VMConfig represents virtual machine configuration aligned with CatalogVm
type VMConfig struct {
	Ram          int     `json:"ram" binding:"required,min=1,max=32"`
	Cpu          int     `json:"cpu" binding:"required,min=1,max=32"`
	Os           string  `json:"os" binding:"required"`
	SshPublicKey *string `json:"ssh_public_key,omitempty"`
	SshKeyName   *string `json:"ssh_key_name,omitempty"`
}

// DeploymentStatus represents the status of a deployment
type DeploymentStatus struct {
	Phase         DeploymentPhase `json:"phase"`
	Message       string          `json:"message,omitempty"`
	ReadyReplicas int             `json:"ready_replicas,omitempty"`
	Conditions    []Condition     `json:"conditions,omitempty"`
}

// DeploymentPhase represents the phase of a deployment
type DeploymentPhase string

const (
	DeploymentPhasePending   DeploymentPhase = "pending"
	DeploymentPhaseRunning   DeploymentPhase = "running"
	DeploymentPhaseSucceeded DeploymentPhase = "succeeded"
	DeploymentPhaseFailed    DeploymentPhase = "failed"
	DeploymentPhaseUnknown   DeploymentPhase = "unknown"
)

// Condition represents a deployment condition
type Condition struct {
	Type               string    `json:"type"`
	Status             string    `json:"status"`
	LastTransitionTime time.Time `json:"last_transition_time"`
	Reason             string    `json:"reason,omitempty"`
	Message            string    `json:"message,omitempty"`
}

// ListDeploymentsRequest represents the request for listing deployments (AEP-0132 token-based pagination)
type ListDeploymentsRequest struct {
	Namespace   string         `form:"namespace"`
	Kind        DeploymentKind `form:"kind"`
	MaxPageSize int            `form:"max_page_size,default=20" binding:"min=0,max=100"`
	PageToken   string         `form:"page_token"`
}

// ListDeploymentsResponse represents the response for listing deployments (AEP-0132)
type ListDeploymentsResponse struct {
	Results       []Deployment `json:"results"`
	NextPageToken string       `json:"next_page_token,omitempty"`
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"check_time"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Type      string    `json:"type,omitempty"`
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	Details   string    `json:"details,omitempty"`
	Timestamp time.Time `json:"error_time"`
}

// BuildResourcePath returns the AEP-compliant resource path for a deployment
func BuildResourcePath(id string) string {
	return fmt.Sprintf("deployments/%s", id)
}

// EncodePageToken encodes an offset into an opaque page token
func EncodePageToken(offset int) string {
	return base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}

// DecodePageToken decodes a page token back into an offset
func DecodePageToken(token string) (int, error) {
	if token == "" {
		return 0, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return 0, fmt.Errorf("invalid page token")
	}
	return strconv.Atoi(string(decoded))
}

// BuildDeploymentSelector creates a label selector for a specific deployment ID
func BuildDeploymentSelector(id string) string {
	return fmt.Sprintf("%s=%s,%s=%s", LabelAppID, id, LabelManagedBy, LabelValueManagedBy)
}

// BuildManagedResourceSelector creates a label selector for all managed resources
func BuildManagedResourceSelector() string {
	return fmt.Sprintf("%s=%s", LabelManagedBy, LabelValueManagedBy)
}

// BuildDeploymentLabels creates the standard set of labels for deployment resources
func BuildDeploymentLabels(id, name string) map[string]string {
	return map[string]string{
		LabelAppID:     id,
		LabelApp:       name,
		LabelManagedBy: LabelValueManagedBy,
	}
}
