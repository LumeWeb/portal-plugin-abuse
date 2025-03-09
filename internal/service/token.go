package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/samber/lo"
	"gorm.io/gorm"
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

func NewTokenService() (core.Service, []core.ContextBuilderOption, error) {
	svc := &TokenService{}
	options := []core.ContextBuilderOption{
		func(ctx core.Context) (core.Context, error) {
			svc.BaseService.InitializeBaseService(ctx, svc)
			return ctx, nil
		},
	}
	return svc, options, nil
}

func (s *TokenService) ID() string {
	return svcTypes.TOKEN_SERVICE
}

func (s *TokenService) GenerateToken(caseID, reporterID uint, validDays int) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		s.logger.Error("Failed to generate token", zap.Error(err))
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	hashed := sha256.Sum256(raw)
	tokenStr := base64.URLEncoding.EncodeToString(raw)

	token := &models.Token{
		CaseID:     caseID,
		ReporterID: reporterID,
		Token:      hashed[:],
		ExpiresAt:  lo.ToPtr(time.Now().Add(time.Duration(validDays) * 24 * time.Hour)),
	}

	if err := db.Create(context.Background(), s.ctx, s.db, token); err != nil {
		s.logger.Error("Failed to store token", zap.Error(err))
		return "", fmt.Errorf("failed to store token: %w", err)
	}

	return tokenStr, nil
}

func (s *TokenService) ValidateToken(token string) (uint, uint, bool) {
	raw, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return 0, 0, false
	}

	hashed := sha256.Sum256(raw)
	var tokenRec models.Token

	err = db.GetByProperties(context.Background(), s.ctx, s.db, map[string]interface{}{
		"token":        hashed[:],
		"revoked_at":   nil,
		"expires_at >": time.Now(),
	}, &tokenRec)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, 0, false
		}
		s.logger.Error("Failed to validate token", zap.Error(err))
		return 0, 0, false
	}

	return tokenRec.CaseID, tokenRec.ReporterID, true
}
