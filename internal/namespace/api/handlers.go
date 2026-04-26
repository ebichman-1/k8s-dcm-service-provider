package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/dcm-project/k8s-service-provider/internal/namespace/models"
	"github.com/dcm-project/k8s-service-provider/internal/namespace/services"
	"go.uber.org/zap"
)

// Handler contains dependencies for HTTP handlers
type Handler struct {
	namespaceService *services.NamespaceService
	logger           *zap.Logger
}

// NewHandler creates a new handler instance
func NewHandler(namespaceService *services.NamespaceService, logger *zap.Logger) *Handler {
	return &Handler{
		namespaceService: namespaceService,
		logger:           logger,
	}
}

// ListNamespaces handles GET /api/v1/namespaces requests (AEP-0132 compliant)
func (h *Handler) ListNamespaces(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Received request to list namespaces")

	w.Header().Set("Content-Type", "application/json")

	// Parse filter query parameter (comma-separated key=value pairs)
	filterStr := r.URL.Query().Get("filter")
	if filterStr == "" {
		h.logger.Error("Empty filter provided")
		h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_ARGUMENT", "Filter parameter is required")
		return
	}

	labels := parseFilterString(filterStr)
	if len(labels) == 0 {
		h.logger.Error("Invalid filter format")
		h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_ARGUMENT", "Filter must contain valid key=value pairs")
		return
	}

	h.logger.Info("Processing label selectors", zap.Any("labels", labels))

	// Get namespaces from service
	response, err := h.namespaceService.GetNamespacesByLabels(r.Context(), labels)
	if err != nil {
		h.logger.Error("Failed to get namespaces from service", zap.Error(err))
		h.writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL", "Failed to fetch namespaces")
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode response", zap.Error(err))
		return
	}

	h.logger.Info("Successfully returned namespaces", zap.Int("count", len(response.Results)))
}

// HealthCheck handles GET /api/v1/health requests
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Received health check request")

	w.Header().Set("Content-Type", "application/json")

	// Check service health
	err := h.namespaceService.HealthCheck(r.Context())

	response := models.HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
	}

	if err != nil {
		h.logger.Error("Health check failed", zap.Error(err))
		response.Status = "unhealthy"
		response.Error = err.Error()
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode health check response", zap.Error(err))
		return
	}
}

// writeErrorResponse writes a standardized error response (AEP-0193 compliant)
func (h *Handler) writeErrorResponse(w http.ResponseWriter, statusCode int, code, message string) {
	response := models.ErrorResponse{
		Code:    code,
		Message: message,
	}

	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode error response", zap.Error(err))
	}
}

// parseFilterString parses a comma-separated key=value filter string into a map
func parseFilterString(filter string) map[string]string {
	labels := make(map[string]string)
	pairs := strings.Split(filter, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if key != "" && value != "" {
				labels[key] = value
			}
		}
	}
	return labels
}

// NotFoundHandler handles 404 errors
func (h *Handler) NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	h.logger.Warn("Endpoint not found", zap.String("path", r.URL.Path))
	w.Header().Set("Content-Type", "application/json")
	h.writeErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "The requested endpoint does not exist")
}

// MethodNotAllowedHandler handles 405 errors
func (h *Handler) MethodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	h.logger.Warn("Method not allowed",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
	)
	w.Header().Set("Content-Type", "application/json")
	h.writeErrorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "The HTTP method is not allowed for this endpoint")
}
