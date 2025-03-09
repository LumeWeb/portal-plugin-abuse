package dto

import (
	z "github.com/Oudwins/zog"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
)

var _ httputil.DTOValidator = (*ReporterCreateRequest)(nil)
var _ httputil.DTOValidator = (*ReporterUpdateRequest)(nil)
var _ httputil.DTORequest[*models.Reporter] = (*ReporterCreateRequest)(nil)
var _ httputil.DTORequest[*models.Reporter] = (*ReporterUpdateRequest)(nil)
var _ httputil.DTOResponse[*models.Reporter] = (*ReporterResponse)(nil)

// ReporterCreateRequest represents the data needed to create a reporter
type ReporterCreateRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

// ReporterUpdateRequest represents the data needed to update a reporter
type ReporterUpdateRequest struct {
	Email *string `json:"email,omitempty"`
	Name  *string `json:"name,omitempty"`
}

func (r *ReporterCreateRequest) Schema() *z.StructSchema {
	return z.Struct(z.Schema{
		"Email": z.String().Required().Email(),
		"Name":  z.String().Required().Min(2),
	})
}

func (r *ReporterUpdateRequest) Schema() *z.StructSchema {
	return z.Struct(z.Schema{
		"Email": z.Ptr(z.String().Optional().Email()),
		"Name":  z.Ptr(z.String().Optional().Min(2)),
	})
}

// ReporterResponse represents the reporter data returned by the API
type ReporterResponse struct {
	BaseResponse
	Email string `json:"email"`
	Name  string `json:"name"`
}

// FromModel converts a model to a response DTO
func (r *ReporterResponse) FromModel(reporter *models.Reporter) error {
	r.BaseResponse = BaseResponse{
		ID:        reporter.ID,
		CreatedAt: reporter.CreatedAt,
		UpdatedAt: reporter.UpdatedAt,
	}
	r.Email = reporter.Email
	r.Name = reporter.Name
	return nil
}

// ToModel converts a create request DTO to a model
func (req *ReporterCreateRequest) ToModel() (*models.Reporter, error) {
	return &models.Reporter{
		Email: req.Email,
		Name:  req.Name,
	}, nil
}

// ToModel converts an update request DTO to a model
func (req *ReporterUpdateRequest) ToModel() (*models.Reporter, error) {
	reporter := &models.Reporter{}

	if req.Email != nil {
		reporter.Email = *req.Email
	}
	if req.Name != nil {
		reporter.Name = *req.Name
	}

	return reporter, nil
}
