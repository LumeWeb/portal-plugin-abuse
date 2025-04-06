package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"github.com/btcsuite/btcutil/base58"
	"strings"
	"time"

	"go.lumeweb.com/portal-plugin-abuse/internal/db"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	svcTypes "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	"go.uber.org/zap"
)

type TokenService struct {
	BaseService
}

func (s *TokenService) GetTokenParts(token string) []string {
	// Split the token into caseID:reporterID:tokenValue format
	parts := strings.SplitN(token, ":", 3)
	if len(parts) != 3 {
		return []string{}
	}
	return parts
}

var _ svcTypes.TokenService = (*TokenService)(nil)

func NewTokenService() (core.Service, []core.ContextBuilderOption, error) {
	svc := &TokenService{}
	return svc, core.ContextOptions(
		core.ContextWithStartupFunc(func(ctx core.Context) error {
			svc.BaseService.InitializeBaseService(ctx, svc)
			return nil
		}),
	), nil
}

func (s *TokenService) ID() string {
	return svcTypes.TOKEN_SERVICE
}

func (s *TokenService) GenerateToken(caseID, reporterID uint, validDays int) (string, time.Time, error) {
	// Generate 18 bytes (144 bits) for ~24-25 character base58 token
	raw := make([]byte, 18)
	if _, err := rand.Read(raw); err != nil {
		s.logger.Error("Failed to generate token", zap.Error(err))
		return "", time.Time{}, db.HandleDBError(err, "GenerateToken", "Token", 0)
	}

	hashed := sha256.Sum256(raw)

	// Use raw base58 encoding without formatting
	tokenStr := base58.Encode(raw)

	expiresAt := time.Now().Add(time.Duration(validDays) * 24 * time.Hour)

	token := &models.Token{
		CaseID:     caseID,
		ReporterID: reporterID,
		Token:      hashed[:],
		ExpiresAt:  &expiresAt,
	}

	if err := db.Create(context.Background(), s.ctx, s.db, token); err != nil {
		s.logger.Error("Failed to store token", zap.Error(err))
		return "", time.Time{}, fmt.Errorf("failed to store token: %w", err)
	}

	return tokenStr, expiresAt, nil
}

// ValidateToken checks if an access token is valid
func (s *TokenService) ValidateToken(token string) (uint, uint, bool) {
	// Remove any whitespace but keep all characters
	token = strings.ReplaceAll(token, " ", "")

	raw := base58.Decode(token)
	if raw == nil || len(raw) != 18 {
		return 0, 0, false
	}
	hashed := sha256.Sum256(raw)

	var tokenRec models.Token
	err := db.GetByProperties[models.Token](context.Background(), s.ctx, s.db, map[string]any{
		"token":      hashed[:],
		"revoked_at": nil,
	}, &tokenRec, /*, func(db *gorm.DB) *gorm.DB {
			return db.Where("expires_at > ?", time.Now())
		}*/)

	if err != nil {
		if db.IsRecordNotFound(err) {
			return 0, 0, false
		}
		s.logger.Error("Failed to validate token", zap.Error(err))
		return 0, 0, false // Don't leak DB errors for security
	}

	return tokenRec.CaseID, tokenRec.ReporterID, true
}
