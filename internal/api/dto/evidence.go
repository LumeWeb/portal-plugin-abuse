package dto

import (
	z "github.com/Oudwins/zog"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"gorm.io/datatypes"
)

var _ httputil.DTOValidator = (*EvidenceCreateRequest)(nil)
var _ httputil.DTOValidator = (*EvidenceUpdateRequest)(nil)
var _ httputil.DTORequest[*models.Evidence] = (*EvidenceCreateRequest)(nil)
var _ httputil.DTORequest[*models.Evidence] = (*EvidenceUpdateRequest)(nil)
var _ httputil.DTOResponse[*models.Evidence] = (*EvidenceResponse)(nil)

// EvidenceCreateRequest represents the data needed to create evidence
type EvidenceCreateRequest struct {
	FileName    string         `json:"file_name"`
	ContentType string         `json:"content_type"`
	Source      string         `json:"source"`
	Description string         `json:"description"`
	Metadata    datatypes.JSON `json:"metadata"`
}

// EvidenceUpdateRequest represents the data needed to update evidence
type EvidenceUpdateRequest struct {
	FileName    *string         `json:"file_name,omitempty"`
	ContentType *string         `json:"content_type,omitempty"`
	Source      *string         `json:"source,omitempty"`
	Description *string         `json:"description,omitempty"`
	Metadata    *datatypes.JSON `json:"metadata,omitempty"`
	FileSize    *int64          `json:"file_size,omitempty"`
}

// EvidenceResponse represents the evidence data returned by the API
type EvidenceResponse struct {
	BaseResponse
	CaseID      uint                  `json:"case_id"`
	SubmitterID uint                  `json:"submitter_id"`
	FileName    string                `json:"file_name"`
	ContentType string                `json:"content_type"`
	StorageHash string                `json:"storage_hash"`
	FileSize    int64                 `json:"file_size"`
	Source      models.EvidenceSource `json:"source"`
	Description string                `json:"description"`
	Metadata    datatypes.JSON        `json:"metadata"`
}

func (r *EvidenceCreateRequest) Schema() *z.StructSchema {
	return z.Struct(z.Schema{
		"FileName":    z.String().Required(),
		"ContentType": z.String().Required(),
		"Source": z.String().Required().OneOf([]string{
			string(models.EvidenceSourceEmail),
			string(models.EvidenceSourceWebUpload),
			string(models.EvidenceSourceAPI),
			string(models.EvidenceSourceSystem),
		}),
		"Description": z.String().Optional(),
	})
}

func (r *EvidenceUpdateRequest) Schema() *z.StructSchema {
	return z.Struct(z.Schema{
		"FileName":    z.Ptr(z.String().Optional()),
		"ContentType": z.Ptr(z.String().Optional()),
		"Source": z.Ptr(z.String().Optional().OneOf([]string{
			string(models.EvidenceSourceEmail),
			string(models.EvidenceSourceWebUpload),
			string(models.EvidenceSourceAPI),
			string(models.EvidenceSourceSystem),
		})),
		"Description": z.Ptr(z.String().Optional()),
	})
}

// FromModel converts a model to a response DTO
func (r *EvidenceResponse) FromModel(evidence *models.Evidence) error {
	r.BaseResponse = BaseResponse{
		ID:        evidence.ID,
		CreatedAt: evidence.CreatedAt,
		UpdatedAt: evidence.UpdatedAt,
	}
	r.CaseID = evidence.CaseID
	r.SubmitterID = evidence.SubmitterID
	r.FileName = evidence.FileName
	r.ContentType = evidence.ContentType
	r.FileSize = evidence.FileSize
	r.Source = evidence.Source
	r.Description = evidence.Description
	r.Metadata = evidence.Metadata
	return nil
}

// ToModel converts a create request DTO to a model
func (req *EvidenceCreateRequest) ToModel() (*models.Evidence, error) {
	return &models.Evidence{
		FileName:    req.FileName,
		ContentType: req.ContentType,
		Source:      models.EvidenceSource(req.Source),
		Description: req.Description,
		Metadata:    req.Metadata,
	}, nil
}

// ToModel converts an update request DTO to a model
func (req *EvidenceUpdateRequest) ToModel() (*models.Evidence, error) {
	evidence := &models.Evidence{}

	if req.FileName != nil {
		evidence.FileName = *req.FileName
	}
	if req.ContentType != nil {
		evidence.ContentType = *req.ContentType
	}
	if req.Source != nil {
		evidence.Source = models.EvidenceSource(*req.Source)
	}
	if req.Description != nil {
		evidence.Description = *req.Description
	}
	if req.Metadata != nil {
		evidence.Metadata = *req.Metadata
	}

	return evidence, nil
}
