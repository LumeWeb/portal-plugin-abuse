// Package messages defines the request and response structures for the template plugin API
package messages

import "go.lumeweb.com/portal-plugin-template/internal/db/models"

// ListItemsResponse represents the response for listing items
// It includes pagination information and the items themselves
type ListItemsResponse struct {
	Items []models.Item `json:"items"`  // Array of items
	Total int64        `json:"total"`   // Total number of items
	Page  int          `json:"page"`    // Current page number
	Limit int          `json:"limit"`   // Items per page
}

// CreateItemRequest represents the request body for creating a new item
type CreateItemRequest struct {
	Name        string `json:"name"`        // Required name field
	Description string `json:"description"` // Optional description
}

// UpdateItemRequest represents the request body for updating an existing item
type UpdateItemRequest struct {
	Name        string `json:"name"`        // New name for the item
	Description string `json:"description"` // New description
}

// SearchItemsResponse represents the response for searching items
// It includes the matching items and total count
type SearchItemsResponse struct {
	Items []models.Item `json:"items"`  // Array of matching items
	Total int64        `json:"total"`   // Total number of matches
}

// UploadState represents the current state of an upload operation
type UploadState struct {
	ID        string          `json:"id"`         // Unique identifier for the upload
	Size      uint64         `json:"size"`       // Total size of the upload in bytes
	Uploaded  uint64         `json:"uploaded"`   // Number of bytes uploaded so far
	Started   time.Time      `json:"started"`    // When the upload started
	Completed bool           `json:"completed"`  // Whether the upload is complete
	Hash      string         `json:"hash"`       // Hash of the uploaded content
}

// UploadStatusResponse represents the response for checking upload status
type UploadStatusResponse struct {
	State *UploadState `json:"state"` // Current state of the upload
}
