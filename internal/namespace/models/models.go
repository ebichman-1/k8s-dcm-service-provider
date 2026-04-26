package models

import (
	"fmt"
	"time"
)

// ListNamespacesRequest represents query parameters for listing namespaces (AEP-0132)
type ListNamespacesRequest struct {
	Filter      string `form:"filter"`
	MaxPageSize int    `form:"max_page_size"`
	PageToken   string `form:"page_token"`
}

// Namespace represents a Kubernetes namespace with its labels
type Namespace struct {
	Path   string            `json:"path,omitempty"`
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels"`
}

// BuildNamespacePath returns the AEP-compliant resource path for a namespace
func BuildNamespacePath(name string) string {
	return fmt.Sprintf("namespaces/%s", name)
}

// NamespaceResponse represents the response containing matching namespaces (AEP-0132)
type NamespaceResponse struct {
	Results       []Namespace `json:"results"`
	NextPageToken string      `json:"next_page_token,omitempty"`
}

// ErrorResponse represents an error response (unified with deployment service)
type ErrorResponse struct {
	Type    string `json:"type,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"check_time"`
	Error     string    `json:"error,omitempty"`
}
