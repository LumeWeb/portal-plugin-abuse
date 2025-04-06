package dto

import (
	"go.lumeweb.com/httputil"
)

// AttachmentUploadResponse represents the response for file uploads
type AttachmentUploadResponse struct {
	Message     string `json:"message"`
	FileName    string `json:"filename"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
}

var _ httputil.DTOResponse[any] = (*AttachmentUploadResponse)(nil)

// FromModel is a placeholder to satisfy the DTOResponse interface
func (r *AttachmentUploadResponse) FromModel(model any) error {
	// No-op for now, unless model conversion is needed later
	return nil
}
