package service

import (
	"fmt"
	"github.com/mnako/letters"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.lumeweb.com/portal-plugin-abuse/internal"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/migrations"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	coreMocks "go.lumeweb.com/portal/core/testing/mocks"
	"net/mail"
	"testing"
)

var (
	EmailTestOptions = coreTesting.CombineOptions(
		coreTesting.WithUnregisterPlugin(internal.PLUGIN_NAME),
		coreTesting.WithUnregisterService(typesSvc.EMAIL_SERVICE),
		coreTesting.NewMockPluginBuilder(internal.PLUGIN_NAME).
			WithService(typesSvc.EMAIL_SERVICE, NewEmailService).
			WithMigrations(core.DBMigration{core.DB_TYPE_SQLITE: migrations.GetSQLite()}).
			Build().
			BuilderOption(),
		coreTesting.WithFireBootComplete(true),
	)
)

func TestEmailService_GenerateCaseThreadID(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		emailService := core.GetService[typesSvc.EmailService](ctx, typesSvc.EMAIL_SERVICE)
		assert.NotNil(tb, emailService)

		referenceNumber := "REF456"
		expectedThreadID := fmt.Sprintf("<case.%s.%s@%s>", referenceNumber, ctx.Config().Config().Core.PortalName, ctx.Config().Config().Core.Domain)

		// Act
		threadID := emailService.GenerateCaseThreadID(referenceNumber)

		// Assert
		assert.Equal(tb, expectedThreadID, threadID)
	}, EmailTestOptions)
}

func TestEmailService_SendTemplatedEmail(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockMailerService := core.GetService[*coreMocks.MockMailerService](ctx, core.MAILER_SERVICE)
		mockMailerService.EXPECT().
			TemplateSend(
				"test_template",
				mock.MatchedBy(func(data core.MailerTemplateData) bool {
					return data["key"] == "value"
				}),
				mock.MatchedBy(func(data core.MailerTemplateData) bool {
					return data["key"] == "value"
				}),
				"test@example.com",
			).
			Return(nil).
			Once()

		emailService := core.GetService[typesSvc.EmailService](ctx, typesSvc.EMAIL_SERVICE)
		assert.NotNil(tb, emailService)

		templateName := "test_template"
		to := []string{"test@example.com"}
		templateData := core.MailerTemplateData{"key": "value"}

		// Act
		err := emailService.SendTemplatedEmail(to, templateName, templateData)

		// Assert
		assert.NoError(tb, err)
	}, EmailTestOptions)
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
	}, EmailTestOptions)
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
	}, EmailTestOptions)
}

func TestEmailService_NewEmailService(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Act
		svc, _, err := NewEmailService()

		// Assert
		assert.NoError(tb, err)
		assert.NotNil(tb, svc)
	}, EmailTestOptions)
}

func TestEmailService_IsEmailProcessed(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		emailService := core.GetService[typesSvc.EmailService](ctx, typesSvc.EMAIL_SERVICE).(*EmailServiceDefault)
		assert.NotNil(tb, emailService)

		testMessageID := "<test123@example.com>"
		testEmail := &letters.Email{
			Headers: letters.Headers{
				MessageID: letters.MessageId(testMessageID),
				Subject:   "Test Subject",
				From:      []*mail.Address{{Name: "Test", Address: "test@example.com"}},
			},
			Text: "Test email content",
		}

		// Test with unprocessed email
		processed, err := emailService.IsEmailProcessed(testEmail)
		assert.NoError(tb, err)
		assert.False(tb, processed)

		// Mark email as processed via Message-ID
		hash, err := emailService.hashEmailContent(testEmail)
		assert.NoError(tb, err)
		err = emailService.db.Create(&models.ProcessedEmail{
			MessageID: testMessageID,
			Hash:      hash,
		}).Error
		assert.NoError(tb, err)

		// Test with processed email (Message-ID path)
		processed, err = emailService.IsEmailProcessed(testEmail)
		assert.NoError(tb, err)
		assert.True(tb, processed)

		// Clear Message-ID to test hash path
		testEmail.Headers.MessageID = ""

		// Test with unprocessed email (hash path)
		processed, err = emailService.IsEmailProcessed(testEmail)
		assert.NoError(tb, err)
		assert.False(tb, processed)

		// Mark email as processed via hash
		hash, err = emailService.hashEmailContent(testEmail)
		assert.NoError(tb, err)
		err = emailService.db.Create(&models.ProcessedEmail{
			Hash: hash,
		}).Error
		assert.NoError(tb, err)

		// Test with processed email (hash path)
		processed, err = emailService.IsEmailProcessed(testEmail)
		assert.NoError(tb, err)
		assert.True(tb, processed)
	}, EmailTestOptions)
}
