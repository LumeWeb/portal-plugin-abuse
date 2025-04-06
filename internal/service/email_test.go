package service

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"testing"
)

func TestEmailService_GenerateCaseThreadID(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		emailService := core.GetService[typesSvc.EmailService](ctx, typesSvc.EMAIL_SERVICE)
		assert.NotNil(tb, emailService)

		caseID := uint(123)
		referenceNumber := "REF456"

		// Act
		threadID := emailService.GenerateCaseThreadID(caseID, referenceNumber)

		// Assert
		expectedThreadID := fmt.Sprintf("<case.%s.%s@%s>", referenceNumber, ctx.Config().Config().Core.PortalName, ctx.Config().Config().Core.Domain)
		assert.Equal(tb, expectedThreadID, threadID)
	}, coreTesting.WithService(typesSvc.EMAIL_SERVICE, NewEmailService))
}

func TestEmailService_SendTemplatedEmail(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		emailService := core.GetService[typesSvc.EmailService](ctx, typesSvc.EMAIL_SERVICE)
		assert.NotNil(tb, emailService)

		mockMailer := coreTesting.GetMockMailerService(ctx)
		templateName := "test_template"
		to := []string{"test@example.com"}
		templateData := core.MailerTemplateData{"key": "value"}

		// Set expectations on the mock mailer service
		mockMailer.On("TemplateSend", templateName, templateData, templateData, to[0]).Return(nil)

		// Act
		err := emailService.SendTemplatedEmail(to, templateName, templateData)

		// Assert
		assert.NoError(tb, err)
		mockMailer.AssertExpectations(tb)
	}, coreTesting.WithService(typesSvc.EMAIL_SERVICE, NewEmailService))
}

func TestEmailService_ValidateTemplateData(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		emailService := core.GetService[typesSvc.EmailService](ctx, typesSvc.EMAIL_SERVICE)
		assert.NotNil(tb, emailService)

		emailServiceDefault := emailService.(*EmailServiceDefault)

		templateName := "test_template"
		templateData := core.MailerTemplateData{
			"CaseID":       "123",
			"ReporterName": "Test User",
		}
		requiredFields := []string{"CaseID", "ReporterName"}

		// Act
		err := emailServiceDefault.ValidateTemplateData(templateName, templateData, requiredFields)

		// Assert
		assert.NoError(tb, err)
	}, coreTesting.WithService(typesSvc.EMAIL_SERVICE, NewEmailService))
}

func TestEmailService_ValidateTemplateData_MissingField(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		emailService := core.GetService[typesSvc.EmailService](ctx, typesSvc.EMAIL_SERVICE)
		assert.NotNil(tb, emailService)

		emailServiceDefault := emailService.(*EmailServiceDefault)

		templateName := "test_template"
		templateData := core.MailerTemplateData{
			"CaseID": "123",
		}
		requiredFields := []string{"CaseID", "ReporterName"}

		// Act
		err := emailServiceDefault.ValidateTemplateData(templateName, templateData, requiredFields)

		// Assert
		assert.Error(tb, err)
		assert.Contains(tb, err.Error(), "missing required field: ReporterName")
	}, coreTesting.WithService(typesSvc.EMAIL_SERVICE, NewEmailService))
}

func TestEmailService_NewEmailService(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Act
		svc, _, err := NewEmailService()

		// Assert
		assert.NoError(tb, err)
		assert.NotNil(tb, svc)
	})
}

func TestEmailService_ID(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		emailService := core.GetService[typesSvc.EmailService](ctx, typesSvc.EMAIL_SERVICE)
		assert.NotNil(tb, emailService)

		// Act
		id := emailService.ID()

		// Assert
		assert.Equal(tb, typesSvc.EMAIL_SERVICE, id)
	}, coreTesting.WithService(typesSvc.EMAIL_SERVICE, NewEmailService))
}
