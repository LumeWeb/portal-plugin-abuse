package dto

import (
	"go.lumeweb.com/httputil"
	"time"

	z "github.com/Oudwins/zog"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
)

var _ httputil.DTOValidator = (*CommunicationCreateRequest)(nil)
var _ httputil.DTOValidator = (*CommunicationUpdateRequest)(nil)
var _ httputil.DTORequest[*models.Communication] = (*CommunicationCreateRequest)(nil)
var _ httputil.DTORequest[*models.Communication] = (*CommunicationUpdateRequest)(nil)
var _ httputil.DTOResponse[*models.Communication] = (*CommunicationResponse)(nil)

// CommunicationCreateRequest represents a request to create a communication
type CommunicationCreateRequest struct {
	Content   string `json:"content"`
	Type      string `json:"type"`
	Direction string `json:"direction"`
	ParentID  *uint  `json:"parent_id,omitempty"`
}

// CommunicationUpdateRequest represents a request to update a communication
type CommunicationUpdateRequest struct {
	Content   *string `json:"content,omitempty"`
	Type      *string `json:"type,omitempty"`
	Direction *string `json:"direction,omitempty"`
	ParentID  *uint   `json:"parent_id,omitempty"` // Double pointer to distinguish between unset and explicit null
}

func (r *CommunicationCreateRequest) Schema() *z.StructSchema {
	return z.Struct(z.Schema{
		"Content": z.String().Required().Min(1),
		"Type": z.String().Required().OneOf([]string{
			string(models.CommunicationTypeEmail),
			string(models.CommunicationTypeNote),
			string(models.CommunicationTypeResponse),
		}),
		"Direction": z.String().Required().OneOf([]string{
			string(models.CommunicationDirectionIncoming),
			string(models.CommunicationDirectionOutgoing),
			string(models.CommunicationDirectionInternal),
			string(models.CommunicationDirectionExternal),
		}),
		"ParentID": z.Ptr(z.Int().Optional().GT(0)),
	})
}

func (r *CommunicationUpdateRequest) Schema() *z.StructSchema {
	return z.Struct(z.Schema{
		"Content": z.Ptr(z.String().Optional().Min(1)),
		"Type": z.Ptr(z.String().Optional().OneOf([]string{
			string(models.CommunicationTypeEmail),
			string(models.CommunicationTypeNote),
			string(models.CommunicationTypeResponse),
		})),
		"Direction": z.Ptr(z.String().Optional().OneOf([]string{
			string(models.CommunicationDirectionIncoming),
			string(models.CommunicationDirectionOutgoing),
			string(models.CommunicationDirectionInternal),
			string(models.CommunicationDirectionExternal),
		})),
		"ParentID": z.Ptr(z.Int().Optional().GT(0)),
	})
}

// ToModel converts a create request DTO to a model
func (req *CommunicationCreateRequest) ToModel() (*models.Communication, error) {
	return &models.Communication{
		Type:      models.CommunicationType(req.Type),
		Direction: models.CommunicationDirection(req.Direction),
		Content:   req.Content,
		ParentID:  req.ParentID,
	}, nil
}

// ToModel converts an update request DTO to a model
func (req *CommunicationUpdateRequest) ToModel() (*models.Communication, error) {
	comm := &models.Communication{}

	if req.Content != nil {
		comm.Content = *req.Content
	}
	if req.Type != nil {
		comm.Type = models.CommunicationType(*req.Type)
	}
	if req.Direction != nil {
		comm.Direction = models.CommunicationDirection(*req.Direction)
	}
	if req.ParentID != nil {
		comm.ParentID = req.ParentID
	}

	return comm, nil
}

type CommunicationResponse struct {
	ID        uint      `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Content   string    `json:"content"`
	Type      string    `json:"type"`
	Direction string    `json:"direction"`
	ParentID  *uint     `json:"parent_id,omitempty"`
}

// FromModel converts a model to a response DTO
func (r *CommunicationResponse) FromModel(comm *models.Communication) error {
	r.ID = comm.ID
	r.CreatedAt = comm.CreatedAt
	r.UpdatedAt = comm.UpdatedAt
	r.Content = comm.Content
	r.Type = string(comm.Type)
	r.Direction = string(comm.Direction)
	r.ParentID = comm.ParentID
	return nil
}
