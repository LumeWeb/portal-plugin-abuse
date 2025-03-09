package dto

import (
	z "github.com/Oudwins/zog"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
)

var _ httputil.DTOValidator = (*CreateCaseRequest)(nil)
var _ httputil.DTOValidator = (*UpdateCaseRequest)(nil)
var _ httputil.DTORequest[*models.Case] = (*CreateCaseRequest)(nil)
var _ httputil.DTOResponse[*models.Case] = (*CaseResponse)(nil)

// CaseCreateRequest represents the data needed to create a case
type CreateCaseRequest struct {
	BaseRequest
	Type        string `json:"type"`
	Priority    string `json:"priority"`
	Source      string `json:"source"`
	NeedsReview bool   `json:"needs_review"`
	ReporterID  int    `json:"reporter_id"`
	SubjectID   int    `json:"subject_id"`
}

// CaseUpdateRequest represents the data needed to update a case
type UpdateCaseRequest struct {
	Description *string `json:"description"`
	Type        *string `json:"type,omitempty"`
	Priority    *string `json:"priority,omitempty"`
	Source      *string `json:"source,omitempty"`
	NeedsReview *bool   `json:"needs_review,omitempty"`
	ReporterID  *int    `json:"reporter_id,omitempty"`
	SubjectID   *int    `json:"subject_id,omitempty"`
}

// ToModel converts an update request DTO to a model
func (req *UpdateCaseRequest) ToModel() (*models.Case, error) {
	caseModel := &models.Case{}

	if req.Description != nil {
		caseModel.Description = *req.Description
	}

	if req.Type != nil {
		caseModel.Type = models.CaseType(*req.Type)
	}
	if req.Priority != nil {
		caseModel.Priority = models.CasePriority(*req.Priority)
	}
	if req.Source != nil {
		caseModel.Source = models.ReportSource(*req.Source)
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

	return caseModel, nil
}

func (r *CreateCaseRequest) Schema() *z.StructSchema {
	return z.Struct(z.Schema{
		"Type":        z.String().Required().OneOf([]string{string(models.CaseTypeSpam), string(models.CaseTypeHarassment), string(models.CaseTypeContent), string(models.CaseTypeMalware), string(models.CaseTypeOther)}),
		"Description": z.String().Required().Min(10),
		"Priority":    z.String().Optional().OneOf([]string{string(models.CasePriorityLow), string(models.CasePriorityMedium), string(models.CasePriorityHigh), string(models.CasePriorityCritical)}),
		"Source":      z.String().Optional().OneOf([]string{string(models.ReportSourceWebForm), string(models.ReportSourceEmail), string(models.ReportSourceAPI)}),
		"NeedsReview": z.Bool().Optional(),
		"ReporterID":  z.Int().Required().GT(0),
		"SubjectID":   z.Int().Required().GT(0),
	})
}

func (r *UpdateCaseRequest) Schema() *z.StructSchema {
	return z.Struct(z.Schema{
		"Type":        z.Ptr(z.String().Optional().OneOf([]string{string(models.CaseTypeSpam), string(models.CaseTypeHarassment), string(models.CaseTypeContent), string(models.CaseTypeMalware), string(models.CaseTypeOther)})),
		"Description": z.Ptr(z.String().Optional().Min(10)),
		"Priority":    z.Ptr(z.String().Optional().OneOf([]string{string(models.CasePriorityLow), string(models.CasePriorityMedium), string(models.CasePriorityHigh), string(models.CasePriorityCritical)})),
		"Source":      z.Ptr(z.String().Optional().OneOf([]string{string(models.ReportSourceWebForm), string(models.ReportSourceEmail), string(models.ReportSourceAPI)})),
		"NeedsReview": z.Ptr(z.Bool().Optional()),
		"ReporterID":  z.Ptr(z.Int().Optional().GT(0)),
		"SubjectID":   z.Ptr(z.Int().Optional().GT(0)),
	})
}

// CaseResponse represents the case data returned by the API
type CaseResponse struct {
	BaseResponse
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
	return z.Struct(z.Schema{
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
	BaseResponse
	OldStatus string `json:"old_status"`
	NewStatus string `json:"new_status"`
}

// FromModel converts a Case model to a CaseStatusResponse
func (r *CaseStatusResponse) FromModel(c *models.Case) error {
	r.BaseResponse = BaseResponse{
		ID:        c.ID,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
	r.NewStatus = string(c.Status)
	return nil
}

// FromModel converts a model to a response DTO
func (r *CaseResponse) FromModel(c *models.Case) error {
	r.BaseResponse = BaseResponse{
		ID:        c.ID,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
	r.ReferenceNumber = c.ReferenceNumber
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
