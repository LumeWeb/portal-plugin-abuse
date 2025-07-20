package dto

import (
	z "github.com/Oudwins/zog"
	"github.com/samber/lo"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"time"
)

var _ httputil.DTOValidator = (*CreateCaseRequest)(nil)
var _ httputil.DTOValidator = (*UpdateCaseRequest)(nil)
var _ httputil.DTORequest[*models.Case] = (*CreateCaseRequest)(nil)
var _ httputil.DTOResponse[*models.Case] = (*CaseResponse)(nil)
var _ httputil.DTOResponse[*models.Case] = (*PublicCaseResponse)(nil)

// CaseCreateRequest represents the data needed to create a case
type CreateCaseRequest struct {
	Description string              `json:"description"`
	Type        models.CaseType     `json:"type"`
	Priority    models.CasePriority `json:"priority"`
	Source      models.ReportSource `json:"source"`
	NeedsReview bool                `json:"needs_review"`
	ReporterID  int                 `json:"reporter_id"`
	SubjectID   int                 `json:"subject_id"`
}

// CaseUpdateRequest represents the data needed to update a case
type UpdateCaseRequest struct {
	Description *string              `json:"description"`
	Type        *models.CaseType     `json:"type,omitempty"`
	Status      *models.CaseStatus   `json:"status,omitempty"`
	Priority    *models.CasePriority `json:"priority,omitempty"`
	Source      *models.ReportSource `json:"source,omitempty"`
	NeedsReview *bool                `json:"needs_review,omitempty"`
	ReporterID  *int                 `json:"reporter_id,omitempty"`
	SubjectID   *int                 `json:"subject_id,omitempty"`
}

// ToModel converts an update request DTO to a model
func (req *UpdateCaseRequest) ToModel() (*models.Case, error) {
	caseModel := &models.Case{}

	if req.Description != nil {
		caseModel.Description = *req.Description
	}

	if req.Type != nil {
		caseModel.Type = *req.Type
	}
	if req.Priority != nil {
		caseModel.Priority = *req.Priority
	}
	if req.Source != nil {
		caseModel.Source = *req.Source
	}
	if req.NeedsReview != nil {
		caseModel.NeedsReview = *req.NeedsReview
	}
	if req.ReporterID != nil {
		caseModel.ReporterID = uint(*req.ReporterID)
	}
	if req.SubjectID != nil {
		caseModel.SubjectID = uint(*req.SubjectID)
	}
	if req.Status != nil {
		caseModel.Status = *req.Status
	}

	return caseModel, nil
}

func (r *CreateCaseRequest) Schema() *z.StructSchema {
	return z.Struct(z.Shape{
		"Type":        z.StringLike[models.CaseType]().Required().OneOf(models.ValidCaseTypes),
		"Description": z.String().Required().Min(10),
		"Priority":    z.StringLike[models.CasePriority]().OneOf(models.ValidCasePriorities),
		"Source":      z.StringLike[models.ReportSource]().Optional().OneOf(models.ValidReportSources),
		"NeedsReview": z.Bool().Optional(),
		"ReporterID":  z.Int().Required().GT(0),
		"SubjectID":   z.Int().Required().GT(0),
	})
}

func (r *UpdateCaseRequest) Schema() *z.StructSchema {
	return z.Struct(z.Shape{
		"Type":        z.Ptr(z.StringLike[models.CaseType]().Optional().OneOf(models.ValidCaseTypes)),
		"Description": z.Ptr(z.String().Optional().Min(10)),
		"Priority":    z.Ptr(z.StringLike[models.CasePriority]().Optional().OneOf(models.ValidCasePriorities)),
		"Source":      z.Ptr(z.StringLike[models.ReportSource]().Optional().OneOf(models.ValidReportSources)),
		"NeedsReview": z.Ptr(z.Bool().Optional()),
		"ReporterID":  z.Ptr(z.Int().Optional().GT(0)),
		"SubjectID":   z.Ptr(z.Int().Optional().GT(0)),
		"Status":      z.Ptr(z.StringLike[models.CaseStatus]().Optional().OneOf(models.ValidCaseStatuses)),
	})
}

// CaseResponse represents the full internal case data
type CaseResponse struct {
	ID              uint                    `json:"id"`
	CreatedAt       time.Time               `json:"created_at"`
	UpdatedAt       time.Time               `json:"updated_at"`
	ReferenceNumber string                  `json:"reference_number"`
	Type            string                  `json:"type"`
	Status          string                  `json:"status"`
	Priority        string                  `json:"priority"`
	Description     string                  `json:"description"`
	Source          string                  `json:"source"`
	NeedsReview     bool                    `json:"needs_review"`
	ReporterID      uint                    `json:"reporter_id"`
	SubjectID       uint                    `json:"subject_id"`
	Communications  []CommunicationResponse `json:"communications,omitempty"`
	Scans           []ScanResponse          `json:"scans,omitempty"`
}

// PublicCaseResponse represents user-facing case data
type PublicCaseResponse struct {
	ReferenceNumber string                  `json:"reference_number"`
	Status          string                  `json:"status"`
	Type            string                  `json:"type"`
	Description     string                  `json:"description"`
	Communications  []CommunicationResponse `json:"communications,omitempty"`
	Scans           []ScanResponse          `json:"scans,omitempty"`
	Attachments     []EvidenceResponse      `json:"attachments,omitempty"`
	CreatedAt       time.Time               `json:"created_at"`
	UpdatedAt       time.Time               `json:"updated_at"`
}

// FromModel converts a Case model to a PublicCaseResponse DTO including related data
func (r *PublicCaseResponse) FromModel(c *models.Case) error {
	r.ReferenceNumber = c.ReferenceNumber
	r.Status = string(c.Status)
	r.Type = string(c.Type)
	r.Description = c.Description
	r.CreatedAt = c.CreatedAt
	r.UpdatedAt = c.UpdatedAt

	// Convert communications
	r.Communications = lo.Map(c.Communications, func(comm models.Communication, _ int) CommunicationResponse {
		var dtoComm CommunicationResponse
		if err := dtoComm.FromModel(&comm); err != nil {
			return CommunicationResponse{}
		}
		return dtoComm
	})

	// Convert scans
	r.Scans = lo.Map(c.CaseScans, func(scan models.CaseScan, _ int) ScanResponse {
		var scanDTO ScanResponse
		if err := scanDTO.FromModel(&scan); err != nil {
			return ScanResponse{}
		}
		return scanDTO
	})

	// Convert evidence to attachments
	r.Attachments = lo.Map(c.Evidence, func(evidence models.Evidence, _ int) EvidenceResponse {
		var evidenceDTO EvidenceResponse
		if err := evidenceDTO.FromModel(&evidence); err != nil {
			return EvidenceResponse{}
		}
		return evidenceDTO
	})

	return nil
}

// CaseStatusUpdateRequest represents a request to update a case's status
type CaseStatusUpdateRequest struct {
	Status string `json:"status"`
}

func (r *CaseStatusUpdateRequest) ToModel() (*models.Case, error) {
	return &models.Case{
		Status: models.CaseStatus(r.Status),
	}, nil
}

func (r *CaseStatusUpdateRequest) Schema() *z.StructSchema {
	return z.Struct(z.Shape{
		"Status": z.String().Required().OneOf([]string{
			string(models.CaseStatusNew),
			string(models.CaseStatusInProgress),
			string(models.CaseStatusResolved),
			string(models.CaseStatusClosed),
		}),
	})
}

// CaseStatusResponse represents the response after updating a case status
type CaseStatusResponse struct {
	ID        uint      `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	OldStatus string    `json:"old_status"`
	NewStatus string    `json:"new_status"`
}

// FromModel converts a Case model to a CaseStatusResponse
func (r *CaseStatusResponse) FromModel(c *models.Case) error {
	r.ID = c.ID
	r.CreatedAt = c.CreatedAt
	r.UpdatedAt = c.UpdatedAt
	r.NewStatus = string(c.Status)
	return nil
}

// FromModel converts a model to a response DTO
func (r *CaseResponse) FromModel(c *models.Case) error {
	r.ID = c.ID
	r.CreatedAt = c.CreatedAt
	r.UpdatedAt = c.UpdatedAt
	r.ReferenceNumber = "CASE-" + c.ReferenceNumber // Ensure consistent prefix
	r.Type = string(c.Type)
	r.Status = string(c.Status)
	r.Priority = string(c.Priority)
	r.Source = string(c.Source)
	r.NeedsReview = c.NeedsReview
	r.ReporterID = c.ReporterID
	r.SubjectID = c.SubjectID
	return nil
}

// ToModel converts a create request DTO to a model
func (req *CreateCaseRequest) ToModel() (*models.Case, error) {
	caseModel := &models.Case{
		Type:        models.CaseType(req.Type),
		Description: req.Description,
		Priority:    models.CasePriority(req.Priority),
		Source:      models.ReportSource(req.Source),
		NeedsReview: req.NeedsReview,
		ReporterID:  uint(req.ReporterID),
		SubjectID:   uint(req.SubjectID),
		Status:      models.CaseStatusNew,
	}

	return caseModel, nil
}
