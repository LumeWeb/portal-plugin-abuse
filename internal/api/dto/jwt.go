package dto

import (
	"time"

	"go.lumeweb.com/httputil"
)

var (
	_ httputil.DTOResponse[*JWTResponse] = (*JWTResponse)(nil)
	_ httputil.DTOResponse[*ValidateTokenResponse] = (*ValidateTokenResponse)(nil)
	_ httputil.DTOResponse[*TokenRefreshResponse] = (*TokenRefreshResponse)(nil)
)

type JWTResponse struct {
	AccessToken string    `json:"access_token"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type ValidateTokenResponse struct {
	Valid     bool   `json:"valid"`
	Reference string `json:"reference"`
}

type TokenRefreshResponse struct {
	Token     string `json:"token"`
	ValidDays int    `json:"valid_days"`
}

// FromModel implements DTOResponse interface for JWTResponse
func (r *JWTResponse) FromModel(model *JWTResponse) error {
	*r = *model
	return nil
}

// FromModel implements DTOResponse interface for ValidateTokenResponse 
func (r *ValidateTokenResponse) FromModel(model *ValidateTokenResponse) error {
	*r = *model
	return nil
}

// FromModel implements DTOResponse interface for TokenRefreshResponse
func (r *TokenRefreshResponse) FromModel(model *TokenRefreshResponse) error {
	*r = *model
	return nil
}
