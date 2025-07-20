package dto

import (
	"fmt"
	z "github.com/Oudwins/zog"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal-plugin-abuse/internal/pkg/urlparser"
	"go.lumeweb.com/portal/core"
	"time"
)

var _ httputil.DTOValidator = (*AbuseReportRequest)(nil)
var _ httputil.DTORequest[*models.Case] = (*AbuseReportRequest)(nil)
var _ httputil.DTOResponse[*AbuseReportResponseModel] = (*AbuseReportResponse)(nil)

var dtoLogger = core.NewLogger(nil)

// AbuseReportRequest represents the incoming request for an abuse report
type AbuseReportRequest struct {
	Email             string          `json:"email"`
	Location          string          `json:"location"`
	AbuseType         models.CaseType `json:"abuse_type"`
	Description       string          `json:"description"`
	AdditionalDetails string          `json:"additional_details,omitempty"`
}

// ToCaseModel converts the DTO to a Case model
func (r AbuseReportRequest) ToModel() (*models.Case, error) {
	// Extract and validate multihash from location
	hashes, err := urlparser.ExtractMultihashesFromURL(r.Location, dtoLogger)
	if err != nil {
		return nil, fmt.Errorf("invalid location URL: %w", err)
	}

	if len(hashes) == 0 {
		return nil, fmt.Errorf("no valid multihash found in location URL")
	}

	// Validate the first found multihash
	shash, err := core.ParseStorageHash(hashes[0])
	if err != nil {
		return nil, fmt.Errorf("invalid multihash in location: %w", err)
	}

	caseModel := &models.Case{
		Type:        r.AbuseType,
		Status:      models.CaseStatusNew,
		Priority:    models.CasePriorityMedium,
		Description: r.Description,
		Source:      models.ReportSourceAPI,
		Reporter: models.Reporter{
			Email: r.Email,
			Name:  r.Email, // Default name to email
		},
		Subject: models.Subject{
			Identifier: shash.Multihash(),
			Type:       models.SubjectTypeURL,
			SourceURL:  r.Location,
		},
	}

	return caseModel, nil
}

func (r *AbuseReportRequest) Schema() *z.StructSchema {
	return z.Struct(z.Shape{
		"Email":             z.String().Required().Email(),
		"Location":          z.String().Required().URL(),
		"AbuseType":         z.StringLike[models.CaseType]().Required().OneOf(models.ValidCaseTypes),
		"Description":       z.String().Required().Min(10),
		"AdditionalDetails": z.String().Optional(),
	})
}

// AbuseReportResponse represents the response for an abuse report submission
type AbuseReportResponse struct {
	CaseReference string    `json:"case_reference"`
	AccessToken   string    `json:"access_token,omitempty"`
	ExpiresAt     time.Time `json:"expires_at,omitempty"`
	ReceivedAt    time.Time `json:"received_at,omitempty"`
}

// AbuseReportResponseModel extends the base response with embedded Case model and auth details
type AbuseReportResponseModel struct {
	*models.Case
	AccessToken string
	ExpiresAt   time.Time
}

// FromModel converts a Case model to a response DTO with auth token
func (r *AbuseReportResponse) FromModel(c *AbuseReportResponseModel) error {
	r.CaseReference = "CASE-" + c.ReferenceNumber
	r.ReceivedAt = c.CreatedAt
	r.AccessToken = c.AccessToken
	r.ExpiresAt = c.ExpiresAt
	return nil
}

// AbuseReportError represents an error response
type AbuseReportError struct {
	Code             string            `json:"code"`
	Message          string            `json:"message"`
	ValidationErrors map[string]string `json:"validation_errors,omitempty"`
}
