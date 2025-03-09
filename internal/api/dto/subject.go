package dto

import (
	"fmt"
	z "github.com/Oudwins/zog"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal/core"
)

var _ httputil.DTOValidator = (*SubjectCreateRequest)(nil)
var _ httputil.DTOValidator = (*SubjectUpdateRequest)(nil)
var _ httputil.DTORequest[*models.Subject] = (*SubjectCreateRequest)(nil)
var _ httputil.DTORequest[*models.Subject] = (*SubjectUpdateRequest)(nil)
var _ httputil.DTOResponse[*models.Subject] = (*SubjectResponse)(nil)

// SubjectCreateRequest represents the data needed to create a subject
type SubjectCreateRequest struct {
	Identifier string `json:"identifier"`
	Type       string `json:"type"`
}

// SubjectUpdateRequest represents the data needed to update a subject
type SubjectUpdateRequest struct {
	Identifier *string `json:"identifier,omitempty"`
	Type       *string `json:"type,omitempty"`
}

func (r *SubjectCreateRequest) Schema() *z.StructSchema {
	return z.Struct(z.Schema{
		"Identifier": z.String().Required().Min(3),
		"Type": z.String().Required().OneOf([]string{
			string(models.SubjectTypeHash),
			string(models.SubjectTypeURL),
		}),
	})
}

func (r *SubjectUpdateRequest) Schema() *z.StructSchema {
	return z.Struct(z.Schema{
		"Identifier": z.Ptr(z.String().Optional().Min(3)),
		"Type": z.Ptr(z.String().Optional().OneOf([]string{
			string(models.SubjectTypeHash),
			string(models.SubjectTypeURL),
		})),
	})
}

// SubjectResponse represents the subject data returned by the API
type SubjectResponse struct {
	BaseResponse
	Identifier string `json:"identifier"`
}

// FromModel converts a model to a response DTO
func (r *SubjectResponse) FromModel(subject *models.Subject) error {
	r.BaseResponse = BaseResponse{
		ID:        subject.ID,
		CreatedAt: subject.CreatedAt,
		UpdatedAt: subject.UpdatedAt,
	}
	r.Identifier = subject.Identifier.B58String()
	return nil
}

// ToModel converts a create request DTO to a model
func (req *SubjectCreateRequest) ToModel() (*models.Subject, error) {
	if req.Identifier == "" {
		return nil, fmt.Errorf("identifier cannot be empty")
	}

	// Validate subject type
	validTypes := map[string]bool{
		string(models.SubjectTypeHash): true,
		string(models.SubjectTypeURL):  true,
	}
	if !validTypes[req.Type] {
		return nil, fmt.Errorf("invalid subject type: %s", req.Type)
	}

	hash, err := core.ParseStorageHash(req.Identifier)
	if err != nil {
		return nil, err
	}

	return &models.Subject{
		Identifier: hash.Multihash(),
		Type:       models.SubjectType(req.Type),
	}, nil
}

// ToModel converts an update request DTO to a model
func (req *SubjectUpdateRequest) ToModel() (*models.Subject, error) {
	subject := &models.Subject{}

	hash, err := core.ParseStorageHash(*req.Identifier)
	if err != nil {
		return nil, err
	}

	if req.Identifier != nil {
		subject.Identifier = hash.Multihash()
	}
	if req.Type != nil {
		// Validate subject type
		validTypes := map[string]bool{
			string(models.SubjectTypeHash): true,
			string(models.SubjectTypeURL):  true,
		}
		if !validTypes[*req.Type] {
			return nil, fmt.Errorf("invalid subject type: %s", *req.Type)
		}
		subject.Type = models.SubjectType(*req.Type)
	}

	return subject, nil
}
