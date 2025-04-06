package dto

import (
	"fmt"
	z "github.com/Oudwins/zog"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal/core"
	"time"

	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"gorm.io/datatypes"
)

var _ httputil.DTOValidator = (*BlockContentCreateRequest)(nil)
var _ httputil.DTOValidator = (*BlockContentUpdateRequest)(nil)
var _ httputil.DTORequest[*models.BlockList] = (*BlockContentCreateRequest)(nil)
var _ httputil.DTORequest[*models.BlockList] = (*BlockContentUpdateRequest)(nil)
var _ httputil.DTOResponse[*models.BlockList] = (*BlockContentResponse)(nil)

// BlockContentCreateRequest represents the request body for blocking content.
type BlockContentCreateRequest struct {
	Hash        string               `json:"hash"`
	MimeType    string               `json:"mime_type,omitempty"`
	FileName    string               `json:"file_name,omitempty"`
	Size        int64                `json:"size,omitempty"`
	UploaderID  *uint                `json:"uploader_id,omitempty"`
	Reason      models.BlockReason   `json:"reason"`
	Severity    models.BlockSeverity `json:"severity"`
	Action      models.BlockAction   `json:"action"`
	Description string               `json:"description,omitempty"`
	Source      models.BlockSource   `json:"source,omitempty"`
	CaseID      *uint                `json:"case_id,omitempty"`
	ExpiresAt   *time.Time           `json:"expires_at,omitempty"`
	Metadata    *string              `json:"metadata,omitempty"`
}

// BlockContentUpdateRequest represents the request body for updating blocked content.
type BlockContentUpdateRequest struct {
	MimeType    *string               `json:"mime_type,omitempty"`
	FileName    *string               `json:"file_name,omitempty"`
	Size        *int64                `json:"size,omitempty"`
	UploaderID  *uint                 `json:"uploader_id,omitempty"`
	Reason      *models.BlockReason   `json:"reason,omitempty"`
	Severity    *models.BlockSeverity `json:"severity,omitempty"`
	Action      *models.BlockAction   `json:"action,omitempty"`
	Description *string               `json:"description,omitempty"`
	Source      *models.BlockSource   `json:"source,omitempty"`
	CaseID      *uint                 `json:"case_id,omitempty"`
	ExpiresAt   *time.Time            `json:"expires_at,omitempty"`
	Metadata    *datatypes.JSON       `json:"metadata,omitempty"`
}

// ToModel converts the DTO to a BlockList model.
func (r *BlockContentCreateRequest) ToModel() (*models.BlockList, error) {
	decodedHash, err := core.ParseStorageHash(r.Hash)
	if err != nil {
		return nil, fmt.Errorf("invalid hash format: %w", err)
	}

	return &models.BlockList{
		Hash:        decodedHash.Multihash(),
		MimeType:    r.MimeType,
		FileName:    r.FileName,
		Size:        uint64(r.Size),
		UploaderID:  r.UploaderID,
		Reason:      r.Reason,
		Severity:    r.Severity,
		Action:      r.Action,
		Description: r.Description,
		Source:      r.Source,
		CaseID:      r.CaseID,
		ExpiresAt:   r.ExpiresAt,
	}, nil
}

// ToModel converts the DTO to a BlockList model.
func (r *BlockContentUpdateRequest) ToModel() (*models.BlockList, error) {
	block := &models.BlockList{}

	if r.MimeType != nil {
		block.MimeType = *r.MimeType
	}
	if r.FileName != nil {
		block.FileName = *r.FileName
	}
	if r.Size != nil {
		block.Size = uint64(*r.Size)
	}
	if r.UploaderID != nil {
		block.UploaderID = r.UploaderID
	}
	if r.Reason != nil {
		block.Reason = *r.Reason
	}
	if r.Severity != nil {
		block.Severity = *r.Severity
	}
	if r.Action != nil {
		block.Action = *r.Action
	}
	if r.Description != nil {
		block.Description = *r.Description
	}
	if r.Source != nil {
		block.Source = *r.Source
	}
	if r.CaseID != nil {
		block.CaseID = r.CaseID
	}
	if r.ExpiresAt != nil {
		block.ExpiresAt = r.ExpiresAt
	}
	if r.Metadata != nil {
		block.Metadata = *r.Metadata
	}

	return block, nil
}

func (r *BlockContentCreateRequest) Schema() *z.StructSchema {
	return z.Struct(z.Schema{
		"Hash":       z.String().Required(),
		"MimeType":   z.String().Optional(),
		"FileName":   z.String().Optional(),
		"Size":       z.Int64().Optional(),
		"UploaderID": z.Ptr(z.Int().Optional()),
		"Reason": StringLike[models.BlockReason]().Required().OneOf([]models.BlockReason{
			models.BlockReasonMalware,
			models.BlockReasonCsam,
			models.BlockReasonCopyright,
			models.BlockReasonHarassment,
			models.BlockReasonHateSpeech,
			models.BlockReasonSpam,
			models.BlockReasonSystemPolicy,
			models.BlockReasonManual,
		}),
		"Severity": StringLike[models.BlockSeverity]().Required().OneOf([]models.BlockSeverity{
			models.BlockSeverityCritical,
			models.BlockSeverityHigh,
			models.BlockSeverityMedium,
			models.BlockSeverityLow,
		}),
		"Action": StringLike[models.BlockAction]().Required().OneOf([]models.BlockAction{
			models.BlockActionReject,
			models.BlockActionQuarantine,
			models.BlockActionWarn,
			models.BlockActionLog,
		}),
		"Description": z.String().Optional(),
		"Source": StringLike[models.BlockSource]().Optional().OneOf([]models.BlockSource{
			models.BlockSourceScanner,
			models.BlockSourceReport,
			models.BlockSourceAdmin,
			models.BlockSourceExternal,
		}),
		"CaseID":    z.Ptr(z.Int().Optional()),
		"ExpiresAt": z.Ptr(z.Time().Optional()),
		"Metadata":  z.Ptr(z.String().Optional()),
	})
}

func (r *BlockContentUpdateRequest) Schema() *z.StructSchema {
	return z.Struct(z.Schema{
		"MimeType":   z.Ptr(z.String().Optional()),
		"FileName":   z.Ptr(z.String().Optional()),
		"Size":       z.Ptr(z.Int64().Optional()),
		"UploaderID": z.Ptr(z.Int().Optional()),
		"Reason": z.Ptr(z.String().Optional().OneOf([]string{
			string(models.BlockReasonMalware),
			string(models.BlockReasonCsam),
			string(models.BlockReasonCopyright),
			string(models.BlockReasonHarassment),
			string(models.BlockReasonHateSpeech),
			string(models.BlockReasonSpam),
			string(models.BlockReasonSystemPolicy),
			string(models.BlockReasonManual),
		})),
		"Severity": z.Ptr(z.String().Optional().OneOf([]string{
			string(models.BlockSeverityCritical),
			string(models.BlockSeverityHigh),
			string(models.BlockSeverityMedium),
			string(models.BlockSeverityLow),
		})),
		"Action": z.Ptr(z.String().Optional().OneOf([]string{
			string(models.BlockActionReject),
			string(models.BlockActionQuarantine),
			string(models.BlockActionWarn),
			string(models.BlockActionLog),
		})),
		"Description": z.Ptr(z.String().Optional()),
		"Source": z.Ptr(z.String().Optional().OneOf([]string{
			string(models.BlockSourceScanner),
			string(models.BlockSourceReport),
			string(models.BlockSourceAdmin),
			string(models.BlockSourceExternal),
		})),
		"CaseID":    z.Ptr(z.Int().Optional()),
		"ExpiresAt": z.Ptr(z.Time().Optional()),
		"Metadata":  z.Ptr(z.String().Optional()),
	})
}

// BlockContentResponse represents the response body for blocked content.
type BlockContentResponse struct {
	ID          uint                 `json:"id"`
	Hash        string               `json:"hash"`
	MimeType    string               `json:"mime_type,omitempty"`
	FileName    string               `json:"file_name,omitempty"`
	Size        uint64               `json:"size,omitempty"`
	UploaderID  *uint                `json:"uploader_id,omitempty"`
	Reason      models.BlockReason   `json:"reason"`
	Severity    models.BlockSeverity `json:"severity"`
	Action      models.BlockAction   `json:"action"`
	Description string               `json:"description,omitempty"`
	BlockedBy   uint                 `json:"blocked_by"`
	Source      models.BlockSource   `json:"source,omitempty"`
	CaseID      *uint                `json:"case_id,omitempty"`
	ExpiresAt   *time.Time           `json:"expires_at,omitempty"`
	ReviewedAt  *time.Time           `json:"reviewed_at,omitempty"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
	Metadata    datatypes.JSON       `json:"metadata,omitempty"`
}

// FromModel converts a BlockList model to a DTO.
func (r *BlockContentResponse) FromModel(block *models.BlockList) error {
	r.ID = block.ID
	r.MimeType = block.MimeType
	r.FileName = block.FileName
	r.Size = block.Size
	r.UploaderID = block.UploaderID
	r.Reason = block.Reason
	r.Severity = block.Severity
	r.Action = block.Action
	r.Description = block.Description
	r.BlockedBy = block.BlockedBy
	r.Source = block.Source
	r.CaseID = block.CaseID
	r.ExpiresAt = block.ExpiresAt
	r.ReviewedAt = block.ReviewedAt
	r.CreatedAt = block.CreatedAt
	r.UpdatedAt = block.UpdatedAt
	r.Metadata = block.Metadata

	// Convert multihash to string
	if block.Hash != nil {
		r.Hash = block.Hash.String()
	}
	return nil
}

// BlockListResponse represents a list of blocked content.
type BlockListResponse struct {
	Items      []BlockContentResponse `json:"items"`
	TotalCount int64                  `json:"total_count"`
}

// NewBlockListResponse creates a new BlockListResponse.
func NewBlockListResponse(items []BlockContentResponse, totalCount int64) *BlockListResponse {
	return &BlockListResponse{
		Items:      items,
		TotalCount: totalCount,
	}
}

// BlockListFilter represents the filters for listing blocked content.
type BlockListFilter struct {
	Hash     string `json:"hash,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Severity string `json:"severity,omitempty"`
	Action   string `json:"action,omitempty"`
	Source   string `json:"source,omitempty"`
	CaseID   *uint  `json:"case_id,omitempty"`
}
