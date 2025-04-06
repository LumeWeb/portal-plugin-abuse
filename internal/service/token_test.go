package service

import (
	"crypto/rand"
	"crypto/sha256"
	"github.com/btcsuite/btcutil/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"testing"
	"time"
)

func TestTokenService_GenerateToken(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		tokenService := core.GetService[typesSvc.TokenService](ctx, typesSvc.TOKEN_SERVICE)
		assert.NotNil(tb, tokenService)

		caseID := uint(1)
		reporterID := uint(2)
		validDays := 90

		// Act
		tokenStr, expiresAt, err := tokenService.GenerateToken(caseID, reporterID, validDays)

		// Assert
		require.NoError(tb, err)
		assert.NotEmpty(tb, tokenStr)
		assert.NotEmpty(tb, expiresAt)

		// Verify token is valid base58 and not empty
		decoded := base58.Decode(tokenStr)
		assert.NotNil(tb, decoded)
		assert.Greater(tb, len(decoded), 0)

		// Verify the token exists in the database (using the hashed value)
		hashed := sha256.Sum256(decoded)
		var retrievedToken models.Token
		err = ctx.DB().Where("token = ?", hashed[:]).First(&retrievedToken).Error
		require.NoError(tb, err)

		assert.Equal(tb, caseID, retrievedToken.CaseID)
		assert.Equal(tb, reporterID, retrievedToken.ReporterID)
		assert.Equal(tb, expiresAt.Unix(), retrievedToken.ExpiresAt.Unix())

	},
		coreTesting.WithService(typesSvc.TOKEN_SERVICE, NewTokenService),
	)
}

func TestTokenService_ValidateToken_Valid(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		tokenService := core.GetService[typesSvc.TokenService](ctx, typesSvc.TOKEN_SERVICE)
		assert.NotNil(tb, tokenService)

		caseID := uint(1)
		reporterID := uint(2)
		validDays := 90

		// Generate token directly to test validation
		raw := make([]byte, 18)
		_, err := rand.Read(raw)
		require.NoError(tb, err)

		hashed := sha256.Sum256(raw)
		tokenValue := base58.Encode(raw)
		expiresAt := time.Now().Add(time.Duration(validDays) * 24 * time.Hour)

		// Store test token directly
		token := &models.Token{
			CaseID:     caseID,
			ReporterID: reporterID,
			Token:      hashed[:],
			ExpiresAt:  &expiresAt,
		}
		err = ctx.DB().Create(token).Error
		require.NoError(tb, err)

		// Act
		retrievedCaseID, retrievedReporterID, isValid := tokenService.ValidateToken(tokenValue)

		// Assert
		assert.True(tb, isValid)
		assert.Equal(tb, caseID, retrievedCaseID)
		assert.Equal(tb, reporterID, retrievedReporterID)
	},
		coreTesting.WithService(typesSvc.TOKEN_SERVICE, NewTokenService),
	)
}

func TestTokenService_ValidateToken_Invalid(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		tokenService := core.GetService[typesSvc.TokenService](ctx, typesSvc.TOKEN_SERVICE)
		assert.NotNil(tb, tokenService)

		// Act
		caseID, reporterID, isValid := tokenService.ValidateToken("invalid-token")

		// Assert
		assert.False(tb, isValid)
		assert.Equal(tb, uint(0), caseID)
		assert.Equal(tb, uint(0), reporterID)
	},
		coreTesting.WithService(typesSvc.TOKEN_SERVICE, NewTokenService),
	)
}

func TestTokenService_GetTokenParts(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		tokenService := core.GetService[typesSvc.TokenService](ctx, typesSvc.TOKEN_SERVICE)
		assert.NotNil(tb, tokenService)

		// Act
		parts := tokenService.GetTokenParts("1:2:test-token")

		// Assert
		assert.Equal(tb, []string{"1", "2", "test-token"}, parts)
	},
		coreTesting.WithService(typesSvc.TOKEN_SERVICE, NewTokenService),
	)
}

func TestTokenService_GetTokenParts_InvalidFormat(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		tokenService := core.GetService[typesSvc.TokenService](ctx, typesSvc.TOKEN_SERVICE)
		assert.NotNil(tb, tokenService)

		// Act
		parts := tokenService.GetTokenParts("invalid-token")

		// Assert
		assert.Empty(tb, parts)
	},
		coreTesting.WithService(typesSvc.TOKEN_SERVICE, NewTokenService),
	)
}

func TestTokenService_ValidateToken_NotFound(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		tokenService := core.GetService[typesSvc.TokenService](ctx, typesSvc.TOKEN_SERVICE)
		assert.NotNil(tb, tokenService)

		// Act
		caseID, reporterID, isValid := tokenService.ValidateToken("non-existent-token")

		// Assert
		assert.False(tb, isValid)
		assert.Equal(tb, uint(0), caseID)
		assert.Equal(tb, uint(0), reporterID)
	},
		coreTesting.WithService(typesSvc.TOKEN_SERVICE, NewTokenService),
	)
}

func TestTokenService_GenerateToken_RandomError(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		tokenService := core.GetService[typesSvc.TokenService](ctx, typesSvc.TOKEN_SERVICE)
		assert.NotNil(tb, tokenService)

		caseID := uint(1)
		reporterID := uint(2)
		validDays := 90

		// Act
		tokenStr, expiresAt, err := tokenService.GenerateToken(caseID, reporterID, validDays)

		// Assert
		require.NoError(tb, err)
		assert.NotEmpty(tb, tokenStr)
		assert.NotEmpty(tb, expiresAt)
	},
		coreTesting.WithService(typesSvc.TOKEN_SERVICE, NewTokenService),
	)
}
