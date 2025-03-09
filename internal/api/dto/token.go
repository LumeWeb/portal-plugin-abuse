package dto

import (
	"encoding/base64"
	z "github.com/Oudwins/zog"
	"go.lumeweb.com/httputil"
	"time"

	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
)

var _ httputil.DTOValidator = (*TokenCreateRequest)(nil)
var _ httputil.DTOValidator = (*TokenUpdateRequest)(nil)
var _ httputil.DTORequest[*models.Token] = (*TokenCreateRequest)(nil)
var _ httputil.DTORequest[*models.Token] = (*TokenUpdateRequest)(nil)
var _ httputil.DTOResponse[*models.Token] = (*TokenResponse)(nil)

// TokenCreateRequest represents the data needed to create a token
type TokenCreateRequest struct {
	CaseID     uint `json:"case_id"`
	ReporterID uint `json:"reporter_id"`
	ValidDays  int  `json:"valid_days"`
}

// TokenUpdateRequest represents the data needed to update a token
type TokenUpdateRequest struct {
	Revoke *bool `json:"revoke,omitempty"`
}

func (r *TokenCreateRequest) Schema() *z.StructSchema {
	return z.Struct(z.Schema{
		"CaseID":     z.Int().Required().GT(0),
		"ReporterID": z.Int().Required().GT(0),
		"ValidDays":  z.Int().GT(1).LTE(365),
	})
}

func (r *TokenUpdateRequest) Schema() *z.StructSchema {
	return z.Struct(z.Schema{
		"Revoke": z.Ptr(z.Bool().Optional()),
	})
}

// TokenResponse represents the token data returned by the API
type TokenResponse struct {
	ID         uint       `json:"id"`
	Token      string     `json:"token"`
	CaseID     uint       `json:"case_id"`
	ReporterID uint       `json:"reporter_id"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// FromModel converts a model to a response DTO
func (r *TokenResponse) FromModel(token *models.Token) error {
	r.ID = token.ID
	r.Token = base64.URLEncoding.EncodeToString(token.Token)
	r.CaseID = token.CaseID
	r.ReporterID = token.ReporterID
	r.ExpiresAt = token.ExpiresAt
	r.RevokedAt = token.RevokedAt
	r.CreatedAt = token.CreatedAt
	r.UpdatedAt = token.UpdatedAt
	return nil
}

// ToModel converts a create request DTO to a model
func (req *TokenCreateRequest) ToModel() (*models.Token, error) {
	expiresAt := time.Now().Add(time.Duration(req.ValidDays) * 24 * time.Hour)
	return &models.Token{
		CaseID:     req.CaseID,
		ReporterID: req.ReporterID,
		ExpiresAt:  &expiresAt,
	}, nil
}

// ToModel converts an update request DTO to a model
func (req *TokenUpdateRequest) ToModel() (*models.Token, error) {
	token := &models.Token{}
	if req.Revoke != nil && *req.Revoke {
		now := time.Now()
		token.RevokedAt = &now
	}
	return token, nil
}
