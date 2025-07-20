package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mnako/letters"
	"github.com/samber/lo"
	"go.lumeweb.com/portal-plugin-abuse/internal"
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
	"net/mail"
	"strings"
)

const (
	defaultARFReporterEmail = "arf-report@automated.system"
	defaultARFReporterName  = "Automated ARF Report"
)

var _ core.Configurable = (*EmailServiceDefault)(nil)

// emailContent represents the structured content of an email for hashing
type emailContent struct {
	MessageID string   `json:"message_id"`
	Subject   string   `json:"subject"`
	From      []string `json:"from"`
	Text      string   `json:"text"`
	HTML      string   `json:"html"`
}

// EmailServiceDefault implements the EmailService interface
type EmailServiceDefault struct {
	BaseService
	config       *config.EmailConfig
	mailer       core.MailerService
	pipeline     email.Pipeline
	caseSvc      typesSvc.CaseService
	commSvc      typesSvc.CommunicationService
	reporterSvc  typesSvc.ReporterService
	subjectSvc   typesSvc.SubjectService
	blocklistSvc typesSvc.BlockListService
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

func (s *EmailServiceDefault) getOrCreateReporter(email string, name string) (*models.Reporter, error) {
	if s.reporterSvc == nil {
		return nil, fmt.Errorf("reporter service not available")
	}

	// Use default values if no email provided
	if email == "" {
		email = defaultARFReporterEmail
		name = defaultARFReporterName
		s.logger.Info("Using default reporter values",
			zap.String("email", email),
			zap.String("name", name))
	}

	// Try to get existing reporter
	reporter, err := s.reporterSvc.GetByEmail(email)
	if err == nil {
		// Update name if we have one and reporter doesn't
		if name != "" && reporter.Name == "" {
			reporter.Name = name
			if err = s.reporterSvc.Update(reporter); err != nil {
				return nil, err
			}
		}
		return reporter, nil
	}

	if !errors.Is(err, db.ErrRecordNotFound) {
		s.logger.Error("Failed to get reporter by email",
			zap.Error(err),
			zap.String("email", email))
		return nil, db.HandleDBError(err, "GetByEmail", "Reporter", 0)
	}

	// Create new reporter
	reporter = &models.Reporter{
		Email: email,
		Name:  lo.Ternary(name != "", name, email),
	}

	reporter, err = s.reporterSvc.Create(reporter)
	if err != nil {
		s.logger.Error("Failed to create reporter",
			zap.Error(err),
			zap.String("email", email),
			zap.String("name", name))
		return nil, db.HandleDBError(err, "Create", "Reporter", 0)
	}

	s.logger.Info("Created new reporter",
		zap.Uint("reporter_id", reporter.ID),
		zap.String("email", reporter.Email),
		zap.String("name", reporter.Name))

	return reporter, nil
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
				blocklistSvc := core.GetService[typesSvc.BlockListService](ctx, typesSvc.BLOCKLIST_SERVICE)

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
				svc.blocklistSvc = blocklistSvc

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

				// Set callbacks that use our service
				pipeline.SetConfigCallback(func() *config.EmailConfig {
					return svc.config
				})
				pipeline.SetProcessedCheckCallback(func(email *letters.Email) (bool, error) {
					return svc.IsEmailProcessed(email)
				})

				svc.pipeline = pipeline

				if err = pipeline.Start(svc.handleProcessedEmail); err != nil {
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
		if errors.Is(err, email.ErrEmailAlreadyProcessed) {
			return nil
		}
		return err
	}

	var processErr error
	switch {
	case result.IsARF:
		processErr = s.handleARFReport(result.ARFData, result.Case)
	case result.Case != nil:
		processErr = s.handleNewCase(result.Case, result.Email)
	case result.ThreadMatch != nil:
		processErr = s.handleThreadMatch(result.ThreadMatch, data)
	default:
		processErr = fmt.Errorf("unknown email processing result type")
	}

	// Mark email as processed if we got a result with an email
	if result.Email != nil {
		if markErr := s.MarkEmailProcessed(result.Email); markErr != nil {
			s.logger.Error("Failed to mark email as processed",
				zap.String("message_id", string(result.Email.Headers.MessageID)),
				zap.Error(markErr))
			// Combine errors if both processing and marking failed
			if processErr != nil {
				return fmt.Errorf("processing error: %v, marking error: %w", processErr, markErr)
			}
			return markErr
		}
	}

	return processErr
}

func (s *EmailServiceDefault) handleARFReport(arfData *email.ARFReport, caseModel *models.Case) error {
	if arfData == nil {
		return fmt.Errorf("ARF report processing failed - no report data")
	}
	if caseModel == nil {
		return fmt.Errorf("ARF report processing failed - no case data")
	}

	// Get reporter service to create or update reporter
	if s.reporterSvc == nil {
		return fmt.Errorf("reporter service not available")
	}

	// Get or create reporter
	reporter, err := s.getOrCreateReporter(arfData.ReporterEmail, "")
	if err != nil {
		return err
	}

	// Extract hashes first - we require at least one valid hash
	hashSubjects, err := s.extractAndLinkHashSubjects(arfData.FeedbackText)
	if err != nil {
		return fmt.Errorf("error extracting hash subjects: %w", err)
	}
	if len(hashSubjects) == 0 {
		s.logger.Info("Skipping ARF report - no valid hashes found")
		return nil
	}

	// Extract URLs if available (optional)
	urlSubjects, err := s.extractAndLinkURLSubjects(arfData.FeedbackText)
	if err != nil {
		return fmt.Errorf("error extracting URL subjects: %w", err)
	}

	allSubjects := lo.UniqBy(append(hashSubjects, urlSubjects...), func(s *models.Subject) uint { return s.ID })

	// Check reporter trust status once before the loop
	isTrusted, err := s.reporterSvc.IsTrusted(reporter)
	if err != nil {
		s.logger.Error("Failed to check reporter trust status",
			zap.Error(err),
			zap.Uint("reporter_id", reporter.ID))
		return fmt.Errorf("failed to check reporter trust status: %w", err)
	}

	// Create cases for each subject
	for _, subject := range allSubjects {
		// Create a new case model copy for each subject
		caseCopy := *caseModel
		caseCopy.ReporterID = reporter.ID
		caseCopy.Type = s.determineCaseType(arfData.FeedbackType)
		caseCopy.Status = models.CaseStatusNew
		caseCopy.Source = models.ReportSourceEmail
		caseCopy.Description = arfData.FeedbackText
		caseCopy.Priority = s.determinePriority(arfData)
		caseCopy.SubjectID = subject.ID
		caseCopy.NeedsReview = !isTrusted // Use pre-checked trust status

		createdCase, err := s.caseSvc.Create(&caseCopy)
		if err != nil {
			s.logger.Error("Failed to create case", zap.Error(err))
			return db.HandleDBError(err, "Create", "Case", 0)
		}

		// Auto-resolve and block if case doesn't need review
		if !caseCopy.NeedsReview {
			s.autoResolveAndBlock(createdCase, subject, caseCopy.Type, caseCopy.Priority)

			s.logger.Info("Auto-resolved and blocked subject from ARF report",
				zap.Uint("case_id", createdCase.ID),
				zap.Uint("subject_id", createdCase.SubjectID),
				zap.String("feedback_type", arfData.FeedbackType))
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
			arfData.FeedbackType,
			arfData.SourceIP,
			arfData.UserAgent,
			arfData.ArrivalDate,
			arfData.OriginalFrom,
			arfData.OriginalRecipient,
			arfData.OriginalSubject,
			s.formatMachineReadable(arfData.MachineReadable),
			arfData.FeedbackText,
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

		s.logger.Info("Successfully processed ARF report",
			zap.String("feedback_type", arfData.FeedbackType),
			zap.String("case_ref", createdCase.ReferenceNumber),
			zap.Uint("case_id", createdCase.ID))
	}

	return nil
}

func (s *EmailServiceDefault) handleNewCase(caseModel *models.Case, email *letters.Email) error {
	// Get or create reporter from email headers
	var reporter *models.Reporter
	var err error
	if len(email.Headers.From) > 0 {
		reporter, err = s.getOrCreateReporter(email.Headers.From[0].Address, email.Headers.From[0].Name)
		if err != nil {
			return fmt.Errorf("failed to get/create reporter: %w", err)
		}
	} else {
		// Fallback to default reporter if no From header
		reporter, err = s.getOrCreateReporter("", "")
		if err != nil {
			return fmt.Errorf("failed to get default reporter: %w", err)
		}
	}

	// Extract hashes first - we require at least one valid hash
	hashSubjects, err := s.extractAndLinkHashSubjects(email.Text)
	if err != nil {
		return fmt.Errorf("error extracting hash subjects: %w", err)
	}
	if len(hashSubjects) == 0 {
		s.logger.Info("Skipping email - no valid hashes found")
		return nil
	}

	// Extract URLs if available (optional)
	urlSubjects, err := s.extractAndLinkURLSubjects(email.Text)
	if err != nil {
		return fmt.Errorf("error extracting URL subjects: %w", err)
	}

	allSubjects := lo.UniqBy(append(hashSubjects, urlSubjects...), func(s *models.Subject) uint { return s.ID })

	// Check reporter trust status once before the loop
	isTrusted, err := s.reporterSvc.IsTrusted(reporter)
	if err != nil {
		s.logger.Error("Failed to check reporter trust status",
			zap.Error(err),
			zap.Uint("reporter_id", reporter.ID))
		return fmt.Errorf("failed to check reporter trust status: %w", err)
	}

	// Create cases for each subject
	for _, subject := range allSubjects {
		// Create a new case model copy for each subject
		caseCopy := *caseModel
		caseCopy.SubjectID = subject.ID
		caseCopy.ReporterID = reporter.ID
		caseCopy.NeedsReview = !isTrusted // Use pre-checked trust status

		// Create the case
		createdCase, err := s.caseSvc.Create(&caseCopy)
		if err != nil {
			return fmt.Errorf("failed to create case: %w", err)
		}

		// Auto-resolve and block if case doesn't need review
		if !caseCopy.NeedsReview {
			s.autoResolveAndBlock(createdCase, subject, caseCopy.Type, caseCopy.Priority)

			s.logger.Info("Auto-resolved and blocked subject",
				zap.Uint("case_id", createdCase.ID),
				zap.Uint("subject_id", createdCase.SubjectID))
		}

		// Create communication record
		comm := &models.Communication{
			CaseID:    createdCase.ID,
			SenderID:  reporter.ID,
			Type:      models.CommunicationTypeEmail,
			Direction: models.CommunicationDirectionExternal,
			Content:   email.Text,
			ThreadID:  string(email.Headers.MessageID),
		}

		if _, err := s.commSvc.Create(comm); err != nil {
			return fmt.Errorf("failed to create communication: %w", err)
		}

		s.logger.Info("Successfully processed new case",
			zap.String("case_ref", createdCase.ReferenceNumber),
			zap.Uint("case_id", createdCase.ID),
			zap.String("source", string(createdCase.Source)),
			zap.Uint("reporter_id", reporter.ID))
	}

	return nil
}

// ProcessIncomingEmail processes an incoming email through the pipeline
func (s *EmailServiceDefault) ProcessIncomingEmail(ctx context.Context, rawEmail io.Reader) error {
	result, err := s.pipeline.ProcessEmail(ctx, rawEmail)
	if err != nil {
		if errors.Is(err, email.ErrEmailAlreadyProcessed) {
			return nil // Silently skip already processed emails
		}
		return db.HandleDBError(err, "ProcessIncomingEmail", "Email", 0)
	}

	// Mark email as processed if we got a result
	if result != nil && result.Email != nil {
		if err := s.MarkEmailProcessed(result.Email); err != nil {
			s.logger.Error("Failed to mark email as processed",
				zap.String("message_id", string(result.Email.Headers.MessageID)),
				zap.Error(err))
		}
	}

	return nil
}

// handleThreadMatch adds communication to an existing case
func (s *EmailServiceDefault) handleThreadMatch(match *email.ThreadMatch, rawEmail io.Reader) error {
	if s.commSvc == nil {
		return fmt.Errorf("communication service not available")
	}

	// Parse the email content
	_email, err := letters.ParseEmail(rawEmail)
	if err != nil {
		return fmt.Errorf("failed to parse email: %w", err)
	}

	// Get email content
	content := _email.Text
	if content == "" {
		content = _email.HTML
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
	subject := _email.Headers.Subject
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

// autoResolveAndBlock handles auto-resolution and blocking for cases that don't need review
func (s *EmailServiceDefault) autoResolveAndBlock(createdCase *models.Case, subject *models.Subject, caseType models.CaseType, priority models.CasePriority) {
	// Resolve the case
	if err := s.caseSvc.UpdateStatus(createdCase.ID, models.CaseStatusResolved); err != nil {
		s.logger.Error("Failed to auto-resolve case",
			zap.Uint("case_id", createdCase.ID),
			zap.Error(err))
		return
	}

	// Add to block list
	block := &models.BlockList{
		Hash:      subject.Identifier,
		Reason:    internal.CaseTypeToBlockReason(caseType),
		Severity:  internal.CasePriorityToBlockSeverity(priority),
		Action:    models.BlockActionReject,
		Source:    models.BlockSourceReport,
		CaseID:    &createdCase.ID,
	}

	if _, err := s.blocklistSvc.BlockContent(block); err != nil {
		s.logger.Error("Failed to add subject to block list",
			zap.Uint("subject_id", createdCase.SubjectID),
			zap.Error(err))
	}
}

// formatMachineReadable formats ARF machine readable data for storage
func (s *EmailServiceDefault) formatMachineReadable(fields map[string]string) string {
	var result strings.Builder
	for k, v := range fields {
		result.WriteString(fmt.Sprintf("%s: %s\n", k, v))
	}
	return result.String()
}

func (s *EmailServiceDefault) GenerateCaseThreadID(referenceNumber string) string {
	domain := s.ctx.Config().Config().Core.Domain
	return fmt.Sprintf("<case.%s.%s@%s>", referenceNumber, s.ctx.Config().Config().Core.PortalName, domain)
}

func (s *EmailServiceDefault) IsEmailProcessed(email *letters.Email) (bool, error) {
	// First try using Message-ID if available
	if email.Headers.MessageID != "" {
		var processed models.ProcessedEmail
		result := s.db.Where("message_id = ?", email.Headers.MessageID).First(&processed)
		if result.Error == nil {
			return true, nil
		}
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return false, fmt.Errorf("error checking Message-ID: %w", result.Error)
		}
	}

	// Fall back to structured hash of email content
	hash, err := s.hashEmailContent(email)
	if err != nil {
		return false, fmt.Errorf("failed to hash email content: %w", err)
	}

	var processed models.ProcessedEmail
	result := s.db.Where("hash = ?", hash).First(&processed)
	if result.Error == nil {
		return true, nil
	}
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, fmt.Errorf("error checking email hash: %w", result.Error)
}

// hashEmailContent creates a consistent hash of the email's structured content
// MarkEmailProcessed records an email as processed to prevent duplicate handling
func (s *EmailServiceDefault) MarkEmailProcessed(email *letters.Email) error {
	hash, err := s.hashEmailContent(email)
	if err != nil {
		return fmt.Errorf("failed to hash email content: %w", err)
	}

	processed := models.ProcessedEmail{
		MessageID: string(email.Headers.MessageID),
		Hash:      hash,
	}

	if err := s.db.Create(&processed).Error; err != nil {
		return fmt.Errorf("failed to mark email as processed: %w", err)
	}

	return nil
}

func (s *EmailServiceDefault) hashEmailContent(email *letters.Email) ([]byte, error) {
	content := emailContent{
		MessageID: string(email.Headers.MessageID),
		Subject:   email.Headers.Subject,
		Text:      email.Text,
		HTML:      email.HTML,
	}

	// Convert From addresses to strings using lo.Map
	content.From = lo.Map(email.Headers.From, func(from *mail.Address, _ int) string {
		return from.String()
	})

	// Marshal to JSON for consistent hashing
	jsonData, err := json.Marshal(content)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal email content: %w", err)
	}

	hash := sha256.Sum256(jsonData)
	return hash[:], nil
}

// extractAndLinkURLSubjects extracts hashes from URLs in email content and links them to case
func (s *EmailServiceDefault) extractAndLinkURLSubjects(emailContent string) ([]*models.Subject, error) {
	if s.subjectSvc == nil {
		s.logger.Error("Subject service not available")
		return nil, fmt.Errorf("subject service not available")
	}

	// Create temporary email structure for content extraction
	feedbackEmail := &letters.Email{
		Text: emailContent,
	}

	// Extract URLs
	contentExtractor := email.NewContentExtractor(s.logger)
	urls := contentExtractor.ExtractURLs(feedbackEmail)

	subjects := make([]*models.Subject, 0, len(urls)) // Pre-allocate slice

	for _, url := range urls {
		feedbackEmail = &letters.Email{
			Text: url,
		}
		// Try to extract hashes from URL path
		hashes := contentExtractor.ExtractHashes(feedbackEmail)
		if len(hashes) == 0 {
			s.logger.Debug("No hashes found in URL",
				zap.String("url", url))
			continue
		}

		// Process each found hash
		for _, hash := range hashes {
			// Create subject for the hash if found
			sHash, err := core.ParseStorageHash(hash)
			if err != nil {
				s.logger.Warn("Failed to parse hash from URL",
					zap.String("url", url),
					zap.String("hash", hash),
					zap.Error(err))
				continue
			}

			hashSubject, err := s.subjectSvc.FindOrCreate(sHash, models.SubjectTypeHash, url)
			if err != nil {
				s.logger.Error("Failed to create subject from hash in URL",
					zap.String("url", url),
					zap.String("hash", hash),
					zap.Error(err))
				continue
			}
			subjects = append(subjects, hashSubject)
		}
	}

	return subjects, nil
}

// extractAndLinkHashSubjects extracts hashes from email content and links them to case
func (s *EmailServiceDefault) extractAndLinkHashSubjects(emailContent string) ([]*models.Subject, error) {
	if s.subjectSvc == nil {
		s.logger.Error("Subject service not available")
		return nil, fmt.Errorf("subject service not available")
	}

	// Create temporary email structure for content extraction
	feedbackEmail := &letters.Email{
		Text: emailContent,
	}

	// Extract hashes
	contentExtractor := email.NewContentExtractor(s.logger)
	hashes := contentExtractor.ExtractHashes(feedbackEmail)

	subjects := make([]*models.Subject, 0, len(hashes)) // Pre-allocate slice

	for _, hash := range hashes {
		sHash, err := core.ParseStorageHash(hash)
		if err != nil {
			s.logger.Error("Failed to parse hash",
				zap.Error(err),
				zap.String("hash", hash))
			return nil, fmt.Errorf("failed to parse hash: %w", err)
		}

		subject, err := s.subjectSvc.FindOrCreate(sHash, models.SubjectTypeHash, "")
		if err != nil {
			s.logger.Error("Failed to create subject from hash",
				zap.Error(err),
				zap.String("hash", hash))
			return nil, fmt.Errorf("failed to create subject from hash: %w", err)
		}
		subjects = append(subjects, subject)
	}

	return subjects, nil
}
