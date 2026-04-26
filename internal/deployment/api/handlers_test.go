package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dcm-project/k8s-service-provider/internal/deployment/models"
	"github.com/dcm-project/k8s-service-provider/internal/deployment/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"k8s.io/utils/ptr"
)

// MockDeploymentService is a mock implementation of the deployment service
type MockDeploymentService struct {
	mock.Mock
}

// Verify that MockDeploymentService implements DeploymentServiceInterface
var _ services.DeploymentServiceInterface = (*MockDeploymentService)(nil)

func (m *MockDeploymentService) CreateDeployment(ctx context.Context, req *models.Deployment, id string) error {
	args := m.Called(ctx, req, id)
	return args.Error(0)
}

func (m *MockDeploymentService) GetDeploymentByID(ctx context.Context, id string) (*models.Deployment, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Deployment), args.Error(1)
}

func (m *MockDeploymentService) UpdateDeployment(ctx context.Context, req *models.Deployment, id string) error {
	args := m.Called(ctx, req, id)
	return args.Error(0)
}

func (m *MockDeploymentService) DeleteDeployment(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockDeploymentService) ListDeployments(ctx context.Context, req *models.ListDeploymentsRequest) (*models.ListDeploymentsResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ListDeploymentsResponse), args.Error(1)
}

func TestCreateDeployment(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		requestBody    interface{}
		setupMock      func(*MockDeploymentService)
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "successful container creation",
			requestBody: models.Deployment{
				Kind: models.DeploymentKindContainer,
				Metadata: models.Metadata{
					Name:      "test-app",
					Namespace: "default",
				},
				Spec: models.ContainerSpec{
					Container: models.ContainerConfig{
						Image:    "nginx:latest",
						Replicas: ptr.To(1),
					},
				},
			},
			setupMock: func(m *MockDeploymentService) {
				m.On("CreateDeployment", mock.Anything, mock.AnythingOfType("*models.Deployment"), mock.AnythingOfType("string")).Return(nil)
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "successful VM creation",
			requestBody: models.Deployment{
				Kind: models.DeploymentKindVM,
				Metadata: models.Metadata{
					Name:      "test-vm",
					Namespace: "default",
				},
				Spec: models.VMSpec{
					VM: models.VMConfig{
						Ram: 4,
						Cpu: 2,
						Os:  "fedora",
					},
				},
			},
			setupMock: func(m *MockDeploymentService) {
				m.On("CreateDeployment", mock.Anything, mock.AnythingOfType("*models.Deployment"), mock.AnythingOfType("string")).Return(nil)
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "invalid request body",
			requestBody:    "invalid json",
			setupMock:      func(m *MockDeploymentService) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "INVALID_REQUEST",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := new(MockDeploymentService)
			tt.setupMock(mockService)

			logger := zap.NewNop()
			handler := &Handler{
				deployService: mockService,
				logger:        logger,
			}

			// Create request
			bodyBytes, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest("POST", "/api/v1/deployments", bytes.NewBuffer(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()

			// Create gin context
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			// Call handler
			handler.CreateDeployment(c)

			// Assertions
			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedBody != "" {
				assert.Contains(t, w.Body.String(), tt.expectedBody)
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestGetDeployment(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		deploymentID   string
		setupMock      func(*MockDeploymentService)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:         "successful get",
			deploymentID: "test-id",
			setupMock: func(m *MockDeploymentService) {
				response := &models.Deployment{
					Path: "deployments/test-id",
					ID:   "test-id",
					Kind: models.DeploymentKindContainer,
					Metadata: models.Metadata{
						Name:      "test-app",
						Namespace: "default",
					},
				}
				m.On("GetDeploymentByID", mock.Anything, "test-id").Return(response, nil)
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "test-id",
		},
		{
			name:         "deployment not found",
			deploymentID: "nonexistent",
			setupMock: func(m *MockDeploymentService) {
				m.On("GetDeploymentByID", mock.Anything, "nonexistent").Return(nil, models.NewErrDeploymentNotFound("nonexistent"))
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   "DEPLOYMENT_NOT_FOUND",
		},
		{
			name:           "missing deployment ID",
			deploymentID:   "",
			setupMock:      func(m *MockDeploymentService) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "MISSING_ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := new(MockDeploymentService)
			tt.setupMock(mockService)

			logger := zap.NewNop()
			handler := &Handler{
				deployService: mockService,
				logger:        logger,
			}

			// Create request
			req, _ := http.NewRequest("GET", "/api/v1/deployments/"+tt.deploymentID, nil)

			// Create response recorder
			w := httptest.NewRecorder()

			// Create gin context
			c, _ := gin.CreateTestContext(w)
			c.Request = req
			c.Params = []gin.Param{{Key: "deployment", Value: tt.deploymentID}}

			// Call handler
			handler.GetDeployment(c)

			// Assertions
			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedBody != "" {
				assert.Contains(t, w.Body.String(), tt.expectedBody)
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestDeleteDeployment(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		deploymentID   string
		queryParams    string
		setupMock      func(*MockDeploymentService)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:         "successful delete with kind",
			deploymentID: "test-id",
			queryParams:  "?kind=container",
			setupMock: func(m *MockDeploymentService) {
				m.On("DeleteDeployment", mock.Anything, "test-id").Return(nil)
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name:         "successful delete without kind (service handles lookup)",
			deploymentID: "test-id",
			queryParams:  "",
			setupMock: func(m *MockDeploymentService) {
				m.On("DeleteDeployment", mock.Anything, "test-id").Return(nil)
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "missing deployment ID",
			deploymentID:   "",
			queryParams:    "",
			setupMock:      func(m *MockDeploymentService) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "MISSING_ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := new(MockDeploymentService)
			tt.setupMock(mockService)

			logger := zap.NewNop()
			handler := &Handler{
				deployService: mockService,
				logger:        logger,
			}

			// Create request
			req, _ := http.NewRequest("DELETE", "/api/v1/deployments/"+tt.deploymentID+tt.queryParams, nil)

			// Create response recorder
			w := httptest.NewRecorder()

			// Create gin context
			c, _ := gin.CreateTestContext(w)
			c.Request = req
			c.Params = []gin.Param{{Key: "deployment", Value: tt.deploymentID}}

			// Call handler
			handler.DeleteDeployment(c)

			// Assertions
			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedBody != "" {
				assert.Contains(t, w.Body.String(), tt.expectedBody)
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestListDeployments(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		queryParams    string
		setupMock      func(*MockDeploymentService)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:        "successful list",
			queryParams: "?max_page_size=10",
			setupMock: func(m *MockDeploymentService) {
				response := &models.ListDeploymentsResponse{
					Results: []models.Deployment{
						{
							Path: "deployments/test-1",
							ID:   "test-1",
							Kind: models.DeploymentKindContainer,
						},
						{
							Path: "deployments/test-2",
							ID:   "test-2",
							Kind: models.DeploymentKindVM,
						},
					},
				}
				m.On("ListDeployments", mock.Anything, mock.AnythingOfType("*models.ListDeploymentsRequest")).Return(response, nil)
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "test-1",
		},
		{
			name:        "filtered list",
			queryParams: "?kind=container&namespace=test",
			setupMock: func(m *MockDeploymentService) {
				response := &models.ListDeploymentsResponse{
					Results: []models.Deployment{},
				}
				m.On("ListDeployments", mock.Anything, mock.AnythingOfType("*models.ListDeploymentsRequest")).Return(response, nil)
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "results",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := new(MockDeploymentService)
			tt.setupMock(mockService)

			logger := zap.NewNop()
			handler := &Handler{
				deployService: mockService,
				logger:        logger,
			}

			// Create request
			req, _ := http.NewRequest("GET", "/api/v1/deployments"+tt.queryParams, nil)

			// Create response recorder
			w := httptest.NewRecorder()

			// Create gin context
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			// Call handler
			handler.ListDeployments(c)

			// Assertions
			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedBody != "" {
				assert.Contains(t, w.Body.String(), tt.expectedBody)
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestHealthCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup
	logger := zap.NewNop()
	handler := &Handler{
		deployService: nil, // Not needed for health check
		logger:        logger,
	}

	// Create request
	req, _ := http.NewRequest("GET", "/api/v1/health", nil)

	// Create response recorder
	w := httptest.NewRecorder()

	// Create gin context
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	// Call handler
	handler.HealthCheck(c)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "healthy")
}
