package jwt

import (
	gjwt "github.com/golang-jwt/jwt/v5"
	"go.lumeweb.com/portal-middleware/auth/jwt"
)

var _ jwt.ClaimSetter = (*AbuseJWTClaims)(nil)

// AbuseJWTClaims represents custom claims for abuse case JWTs
type AbuseJWTClaims struct {
	CaseID     uint `json:"cid"`
	ReporterID uint `json:"rid"`
	*gjwt.RegisteredClaims
}

// SetIssuer sets the issuer claim
func (a *AbuseJWTClaims) SetIssuer(issuer string) {
	a.RegisteredClaims.Issuer = issuer
}

// SetSubject sets the subject claim
func (a *AbuseJWTClaims) SetSubject(subject string) {
	a.RegisteredClaims.Subject = subject
}

// SetExpiresAt sets the expiration time claim
func (a *AbuseJWTClaims) SetExpiresAt(expiresAt *gjwt.NumericDate) {
	a.RegisteredClaims.ExpiresAt = expiresAt
}

// SetAudience sets the audience claim
func (a *AbuseJWTClaims) SetAudience(audience []string) {
	a.RegisteredClaims.Audience = audience
}

// NewAbuseJWTClaims creates a new AbuseJWTClaims with required fields
func NewAbuseJWTClaims(caseID uint, reporterID uint) *AbuseJWTClaims {
	return &AbuseJWTClaims{
		CaseID:           caseID,
		ReporterID:       reporterID,
		RegisteredClaims: &gjwt.RegisteredClaims{},
	}
}
