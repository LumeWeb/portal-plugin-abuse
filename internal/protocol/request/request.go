package request

import (
	"go.lumeweb.com/portal/db/models/data_models"
)

// TemplateRequest represents protocol-specific request data
type TemplateRequest struct {
	data_models.RequestDataModel
	UploadID string `json:"upload_id"`
	Size     uint64 `json:"size"`
}
