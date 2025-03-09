package dto

import (
	z "github.com/Oudwins/zog"
	"go.lumeweb.com/httputil"
	"time"

	"github.com/google/uuid"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
)

var _ httputil.DTOValidator = (*AbuseReportRequest)(nil)
var _ httputil.DTORequest[*models.Case] = (*AbuseReportRequest)(nil)
var _ httputil.DTOResponse[*models.Case] = (*AbuseReportResponse)(nil)

// AbuseReportRequest represents the incoming request for an abuse report
type AbuseReportRequest struct {
	Email             string `json:"email"`
	Location          string `json:"location"`
	AbuseType         string `json:"abuse_type"`
	Description       string `json:"description"`
	AdditionalDetails string `json:"additional_details,omitempty"`
}

// ToCaseModel converts the DTO to a Case model
func (r AbuseReportRequest) ToModel() (*models.Case, error) {
	caseModel := &models.Case{
		ReferenceNumber: uuid.New().String()[:8],
		Type:            models.CaseType(r.AbuseType),
		Status:          models.CaseStatusNew,
		Priority:        models.CasePriorityMedium,
		Description:     r.Description,
		Source:          models.ReportSourceAPI,
		Reporter: models.Reporter{
			Email: r.Email,
			Name:  r.Email, // Default name to email
		},
		Subject: models.Subject{
			Identifier: nil,
			Type:       models.SubjectTypeURL,
			SourceURL:  r.Location,
		},
	}

	return caseModel, nil
}

func (r *AbuseReportRequest) Schema() *z.StructSchema {
	return z.Struct(z.Schema{
		"Email":    z.String().Required().Email(),
		"Location": z.String().Required().URL(),
		"AbuseType": z.String().Required().OneOf([]string{
			"malicious_content",
			"resource_abuse",
			"copyright_violation",
			"phishing_scam",
			"other",
		}),
		"Description":       z.String().Required().Min(10),
		"AdditionalDetails": z.String().Optional(),
	})
}

// AbuseReportResponse represents the response for an abuse report submission
type AbuseReportResponse struct {
	Success            bool      `json:"success"`
	ConfirmationNumber string    `json:"confirmation_number"`
	ReceivedAt         time.Time `json:"received_at"`
	Message            string    `json:"message,omitempty"`
}

// FromModel converts a Case model to a response DTO
func (r *AbuseReportResponse) FromModel(c *models.Case) error {
	r.Success = true
	r.ConfirmationNumber = c.ReferenceNumber
	r.ReceivedAt = c.CreatedAt
	r.Message = "Your abuse report has been successfully submitted."

	return nil
}

// AbuseReportError represents an error response
type AbuseReportError struct {
	Code             string            `json:"code"`
	Message          string            `json:"message"`
	ValidationErrors map[string]string `json:"validation_errors,omitempty"`
}
