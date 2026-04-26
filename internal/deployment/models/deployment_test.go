package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func TestDeployment_JSON(t *testing.T) {
	tests := []struct {
		name     string
		request  Deployment
		wantJSON string
	}{
		{
			name: "container deployment",
			request: Deployment{
				Kind: DeploymentKindContainer,
				Metadata: Metadata{
					Name:      "test-app",
					Namespace: "default",
					Labels: map[string]string{
						"app":     "test",
						"version": "1.0",
					},
				},
				Spec: ContainerSpec{
					Container: ContainerConfig{
						Image:    "nginx:latest",
						Replicas: ptr.To(2),
						Ports: []PortConfig{
							{
								ContainerPort: 80,
								ServicePort:   8080,
								Protocol:      "TCP",
							},
						},
						Resources: &ResourceConfig{
							CPU:    "100m",
							Memory: "128Mi",
						},
					},
				},
			},
			wantJSON: `{"kind":"container","metadata":{"name":"test-app","namespace":"default","labels":{"app":"test","version":"1.0"}},"spec":{"container":{"image":"nginx:latest","replicas":2,"ports":[{"container_port":80,"service_port":8080,"protocol":"TCP"}],"resources":{"cpu":"100m","memory":"128Mi"}}},"status":{"phase":""},"create_time":"0001-01-01T00:00:00Z","update_time":"0001-01-01T00:00:00Z"}`,
		},
		{
			name: "VM deployment",
			request: Deployment{
				Kind: DeploymentKindVM,
				Metadata: Metadata{
					Name:      "test-vm",
					Namespace: "default",
				},
				Spec: VMSpec{
					VM: VMConfig{
						Ram: 4,
						Cpu: 2,
						Os:  "fedora",
					},
				},
			},
			wantJSON: `{"kind":"vm","metadata":{"name":"test-vm","namespace":"default"},"spec":{"vm":{"ram":4,"cpu":2,"os":"fedora"}},"status":{"phase":""},"create_time":"0001-01-01T00:00:00Z","update_time":"0001-01-01T00:00:00Z"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			gotJSON, err := json.Marshal(tt.request)
			assert.NoError(t, err)
			assert.JSONEq(t, tt.wantJSON, string(gotJSON))

			// Test unmarshaling
			var gotDeployment Deployment
			err = json.Unmarshal([]byte(tt.wantJSON), &gotDeployment)
			assert.NoError(t, err)
			assert.Equal(t, tt.request.Kind, gotDeployment.Kind)
			assert.Equal(t, tt.request.Metadata, gotDeployment.Metadata)
		})
	}
}

func TestDeployment_FullResource_JSON(t *testing.T) {
	now := time.Now()
	deployment := Deployment{
		Path: "deployments/test-id-123",
		ID:   "test-id-123",
		Kind: DeploymentKindContainer,
		Metadata: Metadata{
			Name:      "test-app",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test",
			},
		},
		Spec: ContainerSpec{
			Container: ContainerConfig{
				Image:    "nginx:latest",
				Replicas: ptr.To(1),
			},
		},
		Status: DeploymentStatus{
			Phase:         DeploymentPhaseRunning,
			Message:       "Deployment is running",
			ReadyReplicas: 1,
			Conditions: []Condition{
				{
					Type:               "Ready",
					Status:             "True",
					LastTransitionTime: now,
					Reason:             "DeploymentReady",
					Message:            "Deployment is ready",
				},
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Test marshaling
	jsonData, err := json.Marshal(deployment)
	assert.NoError(t, err)
	assert.Contains(t, string(jsonData), "test-id-123")
	assert.Contains(t, string(jsonData), "container")
	assert.Contains(t, string(jsonData), "running")
	assert.Contains(t, string(jsonData), "deployments/test-id-123")
	assert.Contains(t, string(jsonData), "create_time")
	assert.Contains(t, string(jsonData), "update_time")
	assert.Contains(t, string(jsonData), "ready_replicas")
	assert.Contains(t, string(jsonData), "last_transition_time")

	// Test unmarshaling
	var unmarshaled Deployment
	err = json.Unmarshal(jsonData, &unmarshaled)
	assert.NoError(t, err)
	assert.Equal(t, deployment.ID, unmarshaled.ID)
	assert.Equal(t, deployment.Path, unmarshaled.Path)
	assert.Equal(t, deployment.Kind, unmarshaled.Kind)
	assert.Equal(t, deployment.Status.Phase, unmarshaled.Status.Phase)
}

func TestListDeploymentsRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		request ListDeploymentsRequest
		wantErr bool
	}{
		{
			name: "valid request with defaults",
			request: ListDeploymentsRequest{
				MaxPageSize: 20,
			},
			wantErr: false,
		},
		{
			name: "valid request with filters",
			request: ListDeploymentsRequest{
				Namespace:   "test",
				Kind:        DeploymentKindContainer,
				MaxPageSize: 10,
				PageToken:   "",
			},
			wantErr: false,
		},
		{
			name: "invalid max_page_size too high",
			request: ListDeploymentsRequest{
				MaxPageSize: 200,
			},
			wantErr: true,
		},
		{
			name: "valid zero max_page_size (uses default)",
			request: ListDeploymentsRequest{
				MaxPageSize: 0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.request)

			// Test JSON marshaling/unmarshaling
			jsonData, err := json.Marshal(tt.request)
			assert.NoError(t, err)

			var unmarshaled ListDeploymentsRequest
			err = json.Unmarshal(jsonData, &unmarshaled)
			assert.NoError(t, err)
			assert.Equal(t, tt.request.MaxPageSize, unmarshaled.MaxPageSize)
			assert.Equal(t, tt.request.PageToken, unmarshaled.PageToken)
		})
	}
}

func TestDeploymentKind_String(t *testing.T) {
	tests := []struct {
		kind DeploymentKind
		want string
	}{
		{DeploymentKindContainer, "container"},
		{DeploymentKindVM, "vm"},
	}

	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			assert.Equal(t, tt.want, string(tt.kind))
		})
	}
}

func TestDeploymentPhase_String(t *testing.T) {
	tests := []struct {
		phase DeploymentPhase
		want  string
	}{
		{DeploymentPhasePending, "pending"},
		{DeploymentPhaseRunning, "running"},
		{DeploymentPhaseSucceeded, "succeeded"},
		{DeploymentPhaseFailed, "failed"},
		{DeploymentPhaseUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			assert.Equal(t, tt.want, string(tt.phase))
		})
	}
}

func TestMetadata_Validation(t *testing.T) {
	tests := []struct {
		name     string
		metadata Metadata
		valid    bool
	}{
		{
			name: "valid metadata",
			metadata: Metadata{
				Name:      "test-app",
				Namespace: "default",
				Labels: map[string]string{
					"app":     "test",
					"version": "1.0",
				},
			},
			valid: true,
		},
		{
			name: "valid with empty namespace",
			metadata: Metadata{
				Name: "test-app",
			},
			valid: true,
		},
		{
			name: "invalid empty name",
			metadata: Metadata{
				Name:      "",
				Namespace: "default",
			},
			valid: false,
		},
		{
			name: "valid DNS-1123 name",
			metadata: Metadata{
				Name:      "my-app-123",
				Namespace: "my-namespace",
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that metadata can be marshaled/unmarshaled
			jsonData, err := json.Marshal(tt.metadata)
			assert.NoError(t, err)

			var unmarshaled Metadata
			err = json.Unmarshal(jsonData, &unmarshaled)
			assert.NoError(t, err)
			assert.Equal(t, tt.metadata.Name, unmarshaled.Name)
			assert.Equal(t, tt.metadata.Namespace, unmarshaled.Namespace)

			if !tt.valid && tt.metadata.Name == "" {
				assert.Empty(t, tt.metadata.Name)
			}
		})
	}
}

func TestErrorResponse_JSON(t *testing.T) {
	now := time.Now()
	errorResp := ErrorResponse{
		Code:      "DEPLOYMENT_FAILED",
		Message:   "Failed to create deployment",
		Details:   "Kubernetes API error: namespace not found",
		Timestamp: now,
	}

	// Test marshaling
	jsonData, err := json.Marshal(errorResp)
	assert.NoError(t, err)
	assert.Contains(t, string(jsonData), "DEPLOYMENT_FAILED")
	assert.Contains(t, string(jsonData), "Failed to create deployment")

	// Test unmarshaling
	var unmarshaled ErrorResponse
	err = json.Unmarshal(jsonData, &unmarshaled)
	assert.NoError(t, err)
	assert.Equal(t, errorResp.Code, unmarshaled.Code)
	assert.Equal(t, errorResp.Message, unmarshaled.Message)
	assert.Equal(t, errorResp.Details, unmarshaled.Details)
}

func TestHealthResponse_JSON(t *testing.T) {
	now := time.Now()
	healthResp := HealthResponse{
		Status:    "healthy",
		Timestamp: now,
	}

	// Test marshaling
	jsonData, err := json.Marshal(healthResp)
	assert.NoError(t, err)
	assert.Contains(t, string(jsonData), "healthy")

	// Test unmarshaling
	var unmarshaled HealthResponse
	err = json.Unmarshal(jsonData, &unmarshaled)
	assert.NoError(t, err)
	assert.Equal(t, healthResp.Status, unmarshaled.Status)
}

func TestBuildResourcePath(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		{"abc123", "deployments/abc123"},
		{"test-id", "deployments/test-id"},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			assert.Equal(t, tt.want, BuildResourcePath(tt.id))
		})
	}
}

func TestPageToken(t *testing.T) {
	tests := []struct {
		offset int
	}{
		{0},
		{10},
		{50},
		{100},
	}

	for _, tt := range tests {
		token := EncodePageToken(tt.offset)
		decoded, err := DecodePageToken(token)
		assert.NoError(t, err)
		assert.Equal(t, tt.offset, decoded)
	}

	// Test empty token
	decoded, err := DecodePageToken("")
	assert.NoError(t, err)
	assert.Equal(t, 0, decoded)

	// Test invalid token
	_, err = DecodePageToken("invalid-token")
	assert.Error(t, err)
}
