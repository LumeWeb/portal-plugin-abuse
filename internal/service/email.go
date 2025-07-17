package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/mnako/letters"
	"go.lumeweb.com/portal-plugin-abuse/internal/config"
	"go.lumeweb.com/portal-plugin-abuse/internal/db"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal-plugin-abuse/internal/pkg/email"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/portal/event"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"io"
	"strings"
)

const (
	defaultARFReporterEmail = "arf-report@automated.system"
	defaultARFReporterName  = "Automated ARF Report"
)

var _ core.Configurable = (*EmailServiceDefault)(nil)

// EmailServiceDefault implements the EmailService interface
type EmailServiceDefault struct {
	BaseService
	config      *config.EmailConfig
	mailer      core.MailerService
	pipeline    email.Pipeline
	caseSvc     typesSvc.CaseService
	commSvc     typesSvc.CommunicationService
	reporterSvc typesSvc.ReporterService
	subjectSvc  typesSvc.SubjectService
}

func (s *EmailServiceDefault) Config() (any, error) {
	return &config.EmailConfig{}, nil
}

// determineCaseType maps ARF feedback types to case types
func (s *EmailServiceDefault) determineCaseType(feedbackType string) models.CaseType {
	switch strings.ToLower(feedbackType) {
	case "abuse":
		return models.CaseTypeOther
	case "fraud":
		return models.CaseTypeOther
	case "virus":
		return models.CaseTypeIllegalOrHarmfulContent
	case "other":
		return models.CaseTypeOther
	case "not-spam":
		return models.CaseTypeSpam // Misclassified spam reports
	case "spam":
		return models.CaseTypeSpam
	case "dkim":
		return models.CaseTypeOther
	case "spf":
		return models.CaseTypeOther
	case "tls":
		return models.CaseTypeOther
	case "auth-failure":
		return models.CaseTypeOther
	case "harassment":
		return models.CaseTypeHarassment
	default:
		return models.CaseTypeOther
	}
}

func (s *EmailServiceDefault) determinePriority(arfData *email.ARFReport) models.CasePriority {
	// Simple priority calculation based on report type
	if strings.Contains(strings.ToLower(arfData.FeedbackType), "phishing") {
		return models.CasePriorityHigh
	}
	return models.CasePriorityMedium
}

func (s *EmailServiceDefault) getOrCreateReporter(email string) (*models.Reporter, error) {
	reporterSvc := core.GetService[typesSvc.ReporterService](s.ctx, typesSvc.REPORTER_SERVICE)
	if reporterSvc == nil {
		return nil, fmt.Errorf("reporter service not available")
	}

	reporter, err := reporterSvc.GetByEmail(email)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create new reporter
		return reporterSvc.Create(&models.Reporter{
			Email: email,
			Name:  email, // Default name to email
		})
	}
	return reporter, err
}

// Ensure EmailServiceDefault implements the interface
var _ typesSvc.EmailService = (*EmailServiceDefault)(nil)

// NewEmailService creates a new email service
func NewEmailService() (core.Service, []core.ContextBuilderOption, error) {
	svc := &EmailServiceDefault{}
	return svc, core.ContextOptions(
		core.ContextWithStartupFunc(func(ctx core.Context) error {
			svc.BaseService.InitializeBaseService(ctx, svc)

			// Get the email config from context
			emailConfig := core.GetServiceConfig[*config.EmailConfig](ctx, typesSvc.EMAIL_SERVICE)
			svc.config = emailConfig

			core.Listen(ctx, event.EVENT_BOOT_COMPLETE, func(e *core.CoreEvent[event.BootCompleteEvent]) error {
				// Get required services
				mailerSvc := core.GetService[core.MailerService](ctx, core.MAILER_SERVICE)
				caseSvc := core.GetService[typesSvc.CaseService](ctx, typesSvc.CASE_SERVICE)
				commSvc := core.GetService[typesSvc.CommunicationService](ctx, typesSvc.COMMUNICATION_SERVICE)
				reporterSvc := core.GetService[typesSvc.ReporterService](ctx, typesSvc.REPORTER_SERVICE)
				subjectSvc := core.GetService[typesSvc.SubjectService](ctx, typesSvc.SUBJECT_SERVICE)

				if mailerSvc == nil {
					return fmt.Errorf("mailer service is not initialized")
				}
				if caseSvc == nil {
					return fmt.Errorf("case service is not initialized")
				}
				if commSvc == nil {
					return fmt.Errorf("communication service is not initialized")
				}
				if reporterSvc == nil {
					return db.ErrInvalidInput
				}
				if subjectSvc == nil {
					return fmt.Errorf("subject service not available")
				}

				svc.mailer = mailerSvc
				svc.caseSvc = caseSvc
				svc.commSvc = commSvc
				svc.reporterSvc = reporterSvc
				svc.subjectSvc = subjectSvc

				// Initialize pipeline components
				contentExtractor := email.NewContentExtractor(svc.logger)
				contentScorer := email.NewDictionaryContentScorer(svc.logger)
				headerAnalyzer := email.NewHeaderAnalyzer(svc.logger)
				priorityDeterminer := email.NewPriorityDeterminer()
				reviewDecider := email.NewReviewDecider()
				arfProcessor := email.NewARFProcessor(ctx, contentExtractor)
				classifier, err := email.NewClassifier(svc.logger, contentScorer, headerAnalyzer, priorityDeterminer, reviewDecider)
				if err != nil {
					return err
				}
				threadDetector := email.NewThreadDetector(ctx)
				templateProcessor := email.NewTemplateProcessor(ctx, contentExtractor)

				// Create pipeline with dependencies
				pipeline := email.NewPipeline(
					ctx,
					arfProcessor,
					classifier,
					threadDetector,
					templateProcessor,
				)

				// Set config callback that uses our service config
				pipeline.SetConfigCallback(func() *config.EmailConfig {
					return svc.config
				})

				if err := pipeline.Start(svc.handleProcessedEmail); err != nil {
					svc.pipeline = pipeline
					return fmt.Errorf("failed to start pipeline: %w", err)
				}

				return nil
			})
			return nil
		}),
	), nil
}

// ID returns the service identifier
func (s *EmailServiceDefault) ID() string {
	return typesSvc.EMAIL_SERVICE
}

// handleProcessedEmail handles the pipeline processing result
func (s *EmailServiceDefault) handleProcessedEmail(ctx context.Context, data io.Reader) error {
	result, err := s.pipeline.ProcessEmail(ctx, data)
	if err != nil {
		return err
	}

	switch {
	case result.IsARF:
		if result.ARFData == nil {
			return fmt.Errorf("ARF report processing failed - no report data")
		}

		// Get reporter service to create or update reporter
		if s.reporterSvc == nil {
			return fmt.Errorf("reporter service not available")
		}

		// Create or get reporter based on email address
		var reporter *models.Reporter
		if result.ARFData.ReporterEmail != "" {
			reporter, err = s.reporterSvc.GetByEmail(result.ARFData.ReporterEmail)
			if err != nil && !db.IsRecordNotFound(err) {
				s.logger.Error("Failed to get reporter by email", zap.Error(err))
				return db.HandleDBError(err, "GetByEmail", "Reporter", 0)
			}

			// If not found, create a new reporter
			if reporter == nil || db.IsRecordNotFound(err) {
				reporter = &models.Reporter{
					Email: result.ARFData.ReporterEmail,
					Name:  result.ARFData.ReporterEmail,
				}
				reporter, err = s.reporterSvc.Create(reporter)
				if err != nil {
					s.logger.Error("Failed to create reporter", zap.Error(err))
					return db.HandleDBError(err, "Create", "Reporter", 0)
				}
			}
		} else {
			// For ARF reports without reporter email, use generic
			s.logger.Info("Using default reporter values",
				zap.String("email", defaultARFReporterEmail),
				zap.String("name", defaultARFReporterName))

			reporter, err = s.reporterSvc.GetByEmail(defaultARFReporterEmail)
			if err != nil && !db.IsRecordNotFound(err) {
				s.logger.Error("Failed to get generic reporter",
					zap.Error(err),
					zap.String("email", defaultARFReporterEmail))
				return fmt.Errorf("failed to get generic reporter: %w", err)
			}

			// If not found, create generic reporter
			if reporter == nil || db.IsRecordNotFound(err) {
				reporter = &models.Reporter{
					Email: defaultARFReporterEmail,
					Name:  defaultARFReporterName,
				}
				reporter, err = s.reporterSvc.Create(reporter)
				if err != nil {
					s.logger.Error("Failed to create generic reporter",
						zap.Error(err),
						zap.String("email", defaultARFReporterEmail),
						zap.String("name", defaultARFReporterEmail))
					return db.HandleDBError(err, "Create", "Reporter", 0)
				}

				s.logger.Info("Created new generic reporter",
					zap.Uint("reporter_id", reporter.ID),
					zap.String("email", reporter.Email),
					zap.String("name", reporter.Name))
			}
		}

		// Check if reporter is trusted
		isTrusted, err := s.reporterSvc.IsTrusted(reporter)
		if err != nil {
			s.logger.Error("Failed to check reporter trust status",
				zap.Error(err),
				zap.Uint("reporter_id", reporter.ID))
			return fmt.Errorf("failed to check reporter trust status: %w", err)
		}

		// Create the case - only needs review if reporter is not trusted
		caseModel := &models.Case{
			ReporterID:      reporter.ID,
			Type:            s.determineCaseType(result.ARFData.FeedbackType),
			Status:          models.CaseStatusNew,
			Source:          models.ReportSourceEmail,
			Description:     result.ARFData.FeedbackText,
			NeedsReview:     !isTrusted,
			Priority:        models.CasePriorityMedium,
			ReferenceNumber: "", // Will be generated by the service
		}

		createdCase, err := s.caseSvc.Create(caseModel)
		if err != nil {
			s.logger.Error("Failed to create case", zap.Error(err))
			return db.HandleDBError(err, "Create", "Case", 0)
		}

		// Create communication record
		arfDetails := fmt.Sprintf(`
ARF Report Details:
-------------------
Feedback Type: %s
Source IP: %s
User-Agent: %s
Arrival Date: %s

Original From: %s
Original To: %s
Original Subject: %s

Machine Readable Data:
%s

Feedback Text:
%s
`,
			result.ARFData.FeedbackType,
			result.ARFData.SourceIP,
			result.ARFData.UserAgent,
			result.ARFData.ArrivalDate,
			result.ARFData.OriginalFrom,
			result.ARFData.OriginalRecipient,
			result.ARFData.OriginalSubject,
			s.formatMachineReadable(result.ARFData.MachineReadable),
			result.ARFData.FeedbackText,
		)

		comm := &models.Communication{
			CaseID:    createdCase.ID,
			SenderID:  reporter.ID,
			Type:      models.CommunicationTypeEmail,
			Direction: models.CommunicationDirectionExternal,
			Content:   arfDetails,
			ThreadID:  fmt.Sprintf("ARF-REPORT-%d@system", createdCase.ID),
		}

		if _, err := s.commSvc.Create(comm); err != nil {
			return db.HandleDBError(err, "handleThreadMatch", "Communication", 0)
		}

		// Extract and create subjects and link to case
		s.extractAndCreateSubjects(ctx, createdCase.ID, result.ARFData)

		s.logger.Info("Successfully processed ARF report",
			zap.String("feedback_type", result.ARFData.FeedbackType),
			zap.String("case_ref", createdCase.ReferenceNumber),
			zap.Uint("case_id", createdCase.ID))

		return nil

	case result.ThreadMatch != nil:
		// Handle thread match by adding to existing case
		if err := s.handleThreadMatch(ctx, result.ThreadMatch, data); err != nil {
			s.logger.Error("Failed to handle thread match",
				zap.Error(err),
				zap.Uint("case_id", result.ThreadMatch.CaseID))
			return fmt.Errorf("failed to handle thread match: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unhandled pipeline result")
	}
}

// ProcessIncomingEmail processes an incoming email through the pipeline
func (s *EmailServiceDefault) ProcessIncomingEmail(ctx context.Context, rawEmail io.Reader) error {
	_, err := s.pipeline.ProcessEmail(ctx, rawEmail)
	return db.HandleDBError(err, "ProcessIncomingEmail", "Email", 0)
}

// handleThreadMatch adds communication to an existing case
func (s *EmailServiceDefault) handleThreadMatch(ctx context.Context, match *email.ThreadMatch, rawEmail io.Reader) error {
	if s.commSvc == nil {
		return fmt.Errorf("communication service not available")
	}

	// Parse the email content
	email, err := letters.ParseEmail(rawEmail)
	if err != nil {
		return fmt.Errorf("failed to parse email: %w", err)
	}

	// Get email content
	content := email.Text
	if content == "" {
		content = email.HTML
	}

	// Create communication record
	comm := &models.Communication{
		CaseID:    match.CaseID,
		Type:      models.CommunicationTypeEmail,
		Direction: models.CommunicationDirectionIncoming,
		Content:   content,
		ThreadID:  match.Communication.ThreadID, // Use thread ID from matched communication
		ParentID:  &match.Communication.ID,      // Reference parent communication
	}

	if _, err := s.commSvc.Create(comm); err != nil {
		s.logger.Error("Failed to add communication to case",
			zap.Uint("case_id", match.CaseID),
			zap.Error(err))
		return fmt.Errorf("failed to add communication: %w", err)
	}

	// Update case status if needed
	if s.caseSvc != nil {
		if err := s.caseSvc.UpdateStatus(match.CaseID, models.CaseStatusInProgress); err != nil {
			s.logger.Warn("Failed to update case status",
				zap.Uint("case_id", match.CaseID),
				zap.Error(err))
		}
	}

	// Extract subject from email headers
	subject := email.Headers.Subject
	if subject == "" {
		subject = "(no subject)"
	}

	s.logger.Info("Added email to existing case thread",
		zap.Uint("case_id", match.CaseID),
		zap.String("subject", subject),
		zap.String("thread_id", match.Communication.ThreadID))

	return nil
}

// SendTemplatedEmail sends an email using a registered template to all recipients
func (s *EmailServiceDefault) SendTemplatedEmail(to []string, templateName string, data core.MailerTemplateData) error {
	if len(to) == 0 {
		return db.ErrInvalidInput
	}

	var finalErr error

	// Send to each recipient individually
	for _, recipient := range to {
		// Clone data map to avoid mutation between sends
		emailData := make(map[string]interface{}, len(data))
		for k, v := range data {
			emailData[k] = v
		}

		// Send using core mailer template
		err := s.mailer.TemplateSend(
			templateName,
			emailData, // Subject variables
			emailData, // Body variables (same as subject in our case)
			recipient,
		)

		if err != nil {
			s.logger.Error("Failed to send email to recipient",
				zap.String("recipient", recipient),
				zap.Error(err),
			)

			// Record first error but continue sending to others
			if finalErr == nil {
				finalErr = fmt.Errorf("failed to send some emails, first error: %w", err)
			}
		}
	}

	return finalErr
}

// formatMachineReadable formats ARF machine readable data for storage
func (s *EmailServiceDefault) formatMachineReadable(fields map[string]string) string {
	var result strings.Builder
	for k, v := range fields {
		result.WriteString(fmt.Sprintf("%s: %s\n", k, v))
	}
	return result.String()
}

// extractAndCreateSubjects extracts and creates subjects from an ARF report
func (s *EmailServiceDefault) extractAndCreateSubjects(ctx context.Context, caseID uint, report *email.ARFReport) {
	if s.subjectSvc == nil {
		s.logger.Error("Subject service not available")
		return
	}

	// Create a temporary email structure for content extraction
	feedbackEmail := &letters.Email{
		Text: report.FeedbackText,
	}

	// Extract URLs from feedback text
	contentExtractor := email.NewContentExtractor(s.logger)
	urls := contentExtractor.ExtractURLs(feedbackEmail)
	for _, url := range urls {
		subject, err := s.subjectSvc.FindOrCreateByURL(url, models.SubjectTypeURL)
		if err != nil {
			s.logger.Error("Failed to create subject from URL in feedback text",
				zap.Error(err),
				zap.String("url", url))
			continue
		}

		// Link subject to case
		if err := s.caseSvc.LinkSubject(caseID, subject.ID); err != nil {
			s.logger.Error("Failed to link subject to case",
				zap.Uint("case_id", caseID),
				zap.Uint("subject_id", subject.ID),
				zap.Error(err))
		}
	}

	// Extract hashes from feedback text
	hashes := contentExtractor.ExtractHashes(feedbackEmail)
	for _, hash := range hashes {
		sHash, err := core.ParseStorageHash(hash)
		if err != nil {
			s.logger.Error("Failed to create storage hash hash in feedback text",
				zap.Error(err),
				zap.String("hash", hash))
		}
		subject, err := s.subjectSvc.FindOrCreate(sHash, models.SubjectTypeHash)
		if err != nil {
			s.logger.Error("Failed to create subject from hash in feedback text",
				zap.Error(err),
				zap.String("hash", hash))
			continue
		}

		// Link subject to case
		if err := s.caseSvc.LinkSubject(caseID, subject.ID); err != nil {
			s.logger.Error("Failed to link subject to case",
				zap.Uint("case_id", caseID),
				zap.Uint("subject_id", subject.ID),
				zap.Error(err))
		}
	}
}
func (s *EmailServiceDefault) GenerateCaseThreadID(referenceNumber string) string {
	domain := s.ctx.Config().Config().Core.Domain
	return fmt.Sprintf("<case.%s.%s@%s>", referenceNumber, s.ctx.Config().Config().Core.PortalName, domain)
}
