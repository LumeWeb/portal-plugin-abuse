package email

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"regexp"
	"strings"

	"github.com/mnako/letters"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// TemplateProcessor handles detection and processing of known provider email templates
type TemplateProcessor struct {
	ctx              core.Context
	logger           *core.Logger
	contentExtractor *ContentExtractor
	registry         *ProviderTemplateRegistry
}

// GetRegistry returns the provider template registry
func (p *TemplateProcessor) GetRegistry() *ProviderTemplateRegistry {
	return p.registry
}

// SetRegistry sets the provider template registry
func (p *TemplateProcessor) SetRegistry(registry *ProviderTemplateRegistry) {
	p.registry = registry
}

// ProviderTemplate defines the interface for a provider-specific template handler
type ProviderTemplate interface {
	// Match checks if an email matches this provider's template
	Match(email *letters.Email) bool

	// Process processes an email from this provider
	Process(ctx context.Context, email *letters.Email) error

	// ExtractSubjectURLs extracts URLs that might be subjects of the report
	ExtractSubjectURLs(email *letters.Email) []string

	// ExtractReporter extracts reporter information from the email
	ExtractReporter(email *letters.Email) (string, string, error)

	// ExtractCategory extracts the abuse category from the email
	ExtractCategory(email *letters.Email) models.CaseType
}

// NewTemplateProcessor creates a new template processor
func NewTemplateProcessor(ctx core.Context, contentExtractor *ContentExtractor) *TemplateProcessor {
	tp := &TemplateProcessor{
		ctx:              ctx,
		logger:           ctx.NamedLogger("template-processor"),
		contentExtractor: contentExtractor,
		registry:         NewProviderTemplateRegistry(ctx),
	}

	// Register known provider templates
	tp.registerTemplates()

	return tp
}

// registerTemplates registers known provider-specific templates
func (p *TemplateProcessor) registerTemplates() {
	// Register Gmail template
	gmailTemplate := NewGmailTemplate(p.ctx, p.contentExtractor)
	p.registry.RegisterProvider("gmail", gmailTemplate, gmailTemplate.Match, 10)

	// Register Microsoft/Outlook template
	microsoftTemplate := NewMicrosoftTemplate(p.ctx, p.contentExtractor)
	p.registry.RegisterProvider("microsoft", microsoftTemplate, microsoftTemplate.Match, 20)

	// Add other providers as needed
}

// RegisterProviderTemplate allows external registration of new provider templates
func (p *TemplateProcessor) RegisterProviderTemplate(
	providerID string,
	template ProviderTemplate,
	detector ProviderDetector,
	priority int,
) {
	p.registry.RegisterProvider(providerID, template, detector, priority)
}

// DetectProvider detects the email provider from the email content
func (p *TemplateProcessor) DetectProvider(emailData io.Reader) (string, *letters.Email, bool) {
	// Copy the reader to a buffer so we can reuse it
	var buf bytes.Buffer
	teeReader := io.TeeReader(emailData, &buf)

	// Try to parse the email
	parsedEmail, err := letters.ParseEmail(teeReader)
	if err != nil {
		return "", nil, false
	}

	// Use the provider registry to detect the provider
	providerID, matched := p.registry.DetectProvider(&parsedEmail)
	if matched {
		return providerID, &parsedEmail, true
	}

	// No provider detected
	return "", &parsedEmail, false
}

// extractDomain extracts the domain from an email address list
func (p *TemplateProcessor) extractDomain(addresses []*mail.Address) string {
	if len(addresses) == 0 {
		return ""
	}

	// Get the first address
	addr := addresses[0].Address

	// Extract domain (part after @)
	parts := strings.Split(addr, "@")
	if len(parts) < 2 {
		return ""
	}

	return parts[1]
}

// Process processes an email using the detected provider's template
func (p *TemplateProcessor) Process(ctx context.Context, emailData io.Reader, provider string) error {
	// Get the template for this provider from the registry
	template, ok := p.registry.GetTemplate(provider)
	if !ok {
		return fmt.Errorf("no template available for provider: %s", provider)
	}

	// Parse the email
	email, err := letters.ParseEmail(emailData)
	if err != nil {
		p.logger.Error("Failed to parse email for template processing", zap.Error(err))
		return fmt.Errorf("failed to parse email for template processing: %w", err)
	}

	// Process using the provider's template
	return template.Process(ctx, &email)
}

//
// Gmail Template Implementation
//

// GmailTemplate handles Gmail-specific abuse report formats
type GmailTemplate struct {
	ctx              core.Context
	logger           *core.Logger
	contentExtractor *ContentExtractor
}

// NewGmailTemplate creates a new Gmail template processor
func NewGmailTemplate(ctx core.Context, contentExtractor *ContentExtractor) *GmailTemplate {
	return &GmailTemplate{
		ctx:              ctx,
		logger:           ctx.NamedLogger("gmail-template"),
		contentExtractor: contentExtractor,
	}
}

// Match checks if an email matches Gmail's template
func (g *GmailTemplate) Match(email *letters.Email) bool {
	// Check for Gmail-specific patterns in subject and body

	// 1. Check subject for common Gmail abuse report subjects
	subject := email.Headers.Subject
	if strings.Contains(subject, "Gmail Abuse Report") ||
		strings.Contains(subject, "Google Abuse Report") {
		return true
	}

	// 2. Check content for Gmail-specific phrases
	content := email.Text
	if content == "" {
		content = email.HTML
	}

	if strings.Contains(content, "Google received a complaint regarding") ||
		strings.Contains(content, "Gmail has received a complaint") {
		return true
	}

	return false
}

// Process processes a Gmail abuse report
func (g *GmailTemplate) Process(ctx context.Context, email *letters.Email) error {
	// Get reporter service to create or update reporter
	reporterService := core.GetService[typesSvc.ReporterService](g.ctx, typesSvc.REPORTER_SERVICE)
	if reporterService == nil {
		return fmt.Errorf("reporter service not available")
	}

	// Extract reporter information
	reporterEmail, reporterName, err := g.ExtractReporter(email)
	if err != nil {
		g.logger.Warn("Failed to extract reporter information", zap.Error(err))
		reporterEmail = "google-abuse@gmail.com"
		reporterName = "Google Abuse Team"
	}

	// First try to get the reporter by email
	reporter, err := reporterService.GetByEmail(reporterEmail)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		g.logger.Error("Failed to get reporter by email", zap.Error(err))
		return fmt.Errorf("failed to get reporter by email: %w", err)
	}

	// If not found, create a new reporter
	if reporter == nil || errors.Is(err, gorm.ErrRecordNotFound) {
		reporter = &models.Reporter{
			Email: reporterEmail,
			Name:  reporterName, // Use provided name if available
		}
		reporter, err = reporterService.Create(reporter)
		if err != nil {
			g.logger.Error("Failed to create reporter", zap.Error(err))
			return fmt.Errorf("failed to create reporter: %w", err)
		}
	}

	// If we have a name and it's not set, update it
	if reporterName != "" && reporter.Name == "" {
		reporter.Name = reporterName
		if err := reporterService.Update(reporter); err != nil {
			g.logger.Warn("Failed to update reporter name", zap.Error(err))
		}
	}

	// Extract URLs from the email that might be subjects
	subjectURLs := g.ExtractSubjectURLs(email)

	// Determine case type
	caseType := g.ExtractCategory(email)

	// Extract main content
	description := g.contentExtractor.ExtractMainContent(email)

	// Get case service
	caseService := core.GetService[typesSvc.CaseService](g.ctx, typesSvc.CASE_SERVICE)
	if caseService == nil {
		return fmt.Errorf("case service not available")
	}

	// Create a new case
	caseModel := &models.Case{
		ReporterID:      reporter.ID,
		Type:            caseType,
		Status:          models.CaseStatusNew,
		Source:          models.ReportSourceEmail,
		Description:     description,
		NeedsReview:     true,
		Priority:        models.CasePriorityMedium,
		ReferenceNumber: "", // Will be generated by the service
	}

	// Create the case
	caseModel, err = caseService.Create(caseModel)
	if err != nil {
		g.logger.Error("Failed to create case", zap.Error(err))
		return fmt.Errorf("failed to create case: %w", err)
	}

	// Create subjects from the URLs
	subjectService := core.GetService[typesSvc.SubjectService](g.ctx, typesSvc.SUBJECT_SERVICE)
	if subjectService == nil {
		g.logger.Error("Subject service not available")
		return fmt.Errorf("subject service not available")
	}

	for _, url := range subjectURLs {
		subject, err := subjectService.FindOrCreateByURL(url, models.SubjectTypeURL)
		if err != nil {
			g.logger.Error("Failed to create subject from URL", zap.Error(err), zap.String("url", url))
			continue
		}

		// The linking of subjects to cases would need to be done through the Case service
		// or through a new method to be added to the SubjectService interface
		g.logger.Info("Subject created but needs manual linking to case",
			zap.Uint("subject_id", subject.ID),
			zap.Uint("case_id", caseModel.ID))
	}

	// Create a communication record
	communicationService := core.GetService[typesSvc.CommunicationService](g.ctx, typesSvc.COMMUNICATION_SERVICE)
	if communicationService == nil {
		return fmt.Errorf("communication service not available")
	}

	// Create a communication with the original email content
	comm := &models.Communication{
		CaseID:    caseModel.ID,
		SenderID:  reporter.ID,
		Type:      models.CommunicationTypeEmail,
		Direction: models.CommunicationDirectionExternal,
		Content:   email.Text,
		ThreadID:  string(email.Headers.MessageID),
	}

	_, err = communicationService.Create(comm)
	if err != nil {
		g.logger.Error("Failed to create communication", zap.Error(err))
		return fmt.Errorf("failed to create communication: %w", err)
	}

	g.logger.Info("Successfully processed Gmail abuse report",
		zap.String("case_ref", caseModel.ReferenceNumber),
		zap.Uint("case_id", caseModel.ID))

	return nil
}

// ExtractSubjectURLs extracts URLs that might be subjects of the report
func (g *GmailTemplate) ExtractSubjectURLs(email *letters.Email) []string {
	// First use the general URL extractor
	urls := g.contentExtractor.ExtractURLs(email)

	// Then look for Gmail-specific patterns where they indicate reported content
	// For example, they might appear after "Reported URL:" or in a specific section

	content := email.Text
	if content == "" {
		content = email.HTML
	}

	// Look for URLs in specific contexts
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()

		// Check for "Reported URL:" pattern
		if strings.Contains(line, "Reported URL:") ||
			strings.Contains(line, "Reported Content:") {
			// Extract URLs from this line specifically
			urlRegex := regexp.MustCompile(`https?:\/\/(www\.)?[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-zA-Z0-9()]{1,6}\b([-a-zA-Z0-9()@:%_\+.~#?&//=]*)`)
			matches := urlRegex.FindAllString(line, -1)

			for _, url := range matches {
				// Check if already in list
				found := false
				for _, existingURL := range urls {
					if existingURL == url {
						found = true
						break
					}
				}

				if !found {
					urls = append(urls, url)
				}
			}
		}
	}

	return urls
}

// ExtractReporter extracts reporter information from the Gmail abuse report
func (g *GmailTemplate) ExtractReporter(email *letters.Email) (string, string, error) {
	// Default values for Gmail
	reporterEmail := "google-abuse@gmail.com"
	reporterName := "Google Abuse Team"

	// Try to find a better email if available
	if len(email.Headers.From) > 0 {
		fromEmail := email.Headers.From[0].Address
		fromName := email.Headers.From[0].Name

		if fromEmail != "" {
			reporterEmail = fromEmail
		}

		if fromName != "" {
			reporterName = fromName
		}
	}

	return reporterEmail, reporterName, nil
}

// ExtractCategory extracts the abuse category from a Gmail abuse report
func (g *GmailTemplate) ExtractCategory(email *letters.Email) models.CaseType {
	content := email.Text
	if content == "" {
		content = email.HTML
	}

	// Check for category indicators in content
	if strings.Contains(content, "phishing") ||
		strings.Contains(content, "fraud") {
		return models.CaseTypeOther
	}

	if strings.Contains(content, "spam") ||
		strings.Contains(content, "unwanted email") ||
		strings.Contains(content, "bulk") {
		return models.CaseTypeSpam
	}

	if strings.Contains(content, "copyright") ||
		strings.Contains(content, "DMCA") ||
		strings.Contains(content, "infringement") {
		return models.CaseTypeCopyrightViolation
	}

	if strings.Contains(content, "harassment") ||
		strings.Contains(content, "bullying") ||
		strings.Contains(content, "threatening") {
		return models.CaseTypeHarassment
	}

	// Default to Other if no specific category detected
	return models.CaseTypeOther
}

//
// Microsoft/Outlook Template Implementation
//

// MicrosoftTemplate handles Microsoft/Outlook-specific abuse report formats
type MicrosoftTemplate struct {
	ctx              core.Context
	logger           *core.Logger
	contentExtractor *ContentExtractor
}

// NewMicrosoftTemplate creates a new Microsoft template processor
func NewMicrosoftTemplate(ctx core.Context, contentExtractor *ContentExtractor) *MicrosoftTemplate {
	return &MicrosoftTemplate{
		ctx:              ctx,
		logger:           ctx.NamedLogger("microsoft-template"),
		contentExtractor: contentExtractor,
	}
}

// Match checks if an email matches Microsoft's template
func (m *MicrosoftTemplate) Match(email *letters.Email) bool {
	// Check for Microsoft-specific patterns in subject and body

	// 1. Check subject for common Microsoft abuse report subjects
	subject := email.Headers.Subject
	if strings.Contains(subject, "Microsoft Abuse Report") ||
		strings.Contains(subject, "Outlook Abuse Report") {
		return true
	}

	// 2. Check content for Microsoft-specific phrases
	content := email.Text
	if content == "" {
		content = email.HTML
	}

	if strings.Contains(content, "Microsoft received a complaint") ||
		strings.Contains(content, "Outlook has received a complaint") {
		return true
	}

	return false
}

// Process processes a Microsoft abuse report
func (m *MicrosoftTemplate) Process(ctx context.Context, email *letters.Email) error {
	// Get reporter service to create or update reporter
	reporterService := core.GetService[typesSvc.ReporterService](m.ctx, typesSvc.REPORTER_SERVICE)
	if reporterService == nil {
		return fmt.Errorf("reporter service not available")
	}

	// Extract reporter information
	reporterEmail, reporterName, err := m.ExtractReporter(email)
	if err != nil {
		m.logger.Warn("Failed to extract reporter information", zap.Error(err))
		reporterEmail = "abuse@microsoft.com"
		reporterName = "Microsoft Abuse Team"
	}

	// Create or get reporter
	// First try to get the reporter by email
	reporter, err := reporterService.GetByEmail(reporterEmail)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		m.logger.Error("Failed to get reporter by email", zap.Error(err))
		return fmt.Errorf("failed to get reporter by email: %w", err)
	}

	// If not found, create a new reporter
	if reporter == nil || errors.Is(err, gorm.ErrRecordNotFound) {
		reporter = &models.Reporter{
			Email: reporterEmail,
			Name:  reporterName, // Use provided name if available
		}
		reporter, err = reporterService.Create(reporter)
		if err != nil {
			m.logger.Error("Failed to create reporter", zap.Error(err))
			return fmt.Errorf("failed to create reporter: %w", err)
		}
	}

	// If we have a name and it's not set, update it
	if reporterName != "" && reporter.Name == "" {
		reporter.Name = reporterName
		if err := reporterService.Update(reporter); err != nil {
			m.logger.Warn("Failed to update reporter name", zap.Error(err))
		}
	}

	// Extract URLs from the email that might be subjects
	subjectURLs := m.ExtractSubjectURLs(email)

	// Determine case type
	caseType := m.ExtractCategory(email)

	// Extract main content
	description := m.contentExtractor.ExtractMainContent(email)

	// Get case service
	caseService := core.GetService[typesSvc.CaseService](m.ctx, typesSvc.CASE_SERVICE)
	if caseService == nil {
		return fmt.Errorf("case service not available")
	}

	// Create a new case
	caseModel := &models.Case{
		ReporterID:      reporter.ID,
		Type:            caseType,
		Status:          models.CaseStatusNew,
		Source:          models.ReportSourceEmail,
		Description:     description,
		NeedsReview:     true,
		Priority:        models.CasePriorityMedium,
		ReferenceNumber: "", // Will be generated by the service
	}

	// Create the case
	caseModel, err = caseService.Create(caseModel)
	if err != nil {
		m.logger.Error("Failed to create case", zap.Error(err))
		return fmt.Errorf("failed to create case: %w", err)
	}

	// Create subjects from the URLs
	subjectService := core.GetService[typesSvc.SubjectService](m.ctx, typesSvc.SUBJECT_SERVICE)
	if subjectService == nil {
		m.logger.Error("Subject service not available")
		return fmt.Errorf("subject service not available")
	}

	for _, url := range subjectURLs {
		subject, err := subjectService.FindOrCreateByURL(url, models.SubjectTypeURL)
		if err != nil {
			m.logger.Error("Failed to create subject from URL", zap.Error(err), zap.String("url", url))
			continue
		}

		// The linking of subjects to cases would need to be done through the Case service
		// or through a new method to be added to the SubjectService interface
		m.logger.Info("Subject created but needs manual linking to case",
			zap.Uint("subject_id", subject.ID),
			zap.Uint("case_id", caseModel.ID))
	}

	// Create a communication record
	communicationService := core.GetService[typesSvc.CommunicationService](m.ctx, typesSvc.COMMUNICATION_SERVICE)
	if communicationService == nil {
		return fmt.Errorf("communication service not available")
	}

	// Create a communication with the original email content
	comm := &models.Communication{
		CaseID:    caseModel.ID,
		SenderID:  reporter.ID,
		Type:      models.CommunicationTypeEmail,
		Direction: models.CommunicationDirectionExternal,
		Content:   email.Text,
		ThreadID:  string(email.Headers.MessageID),
	}

	_, err = communicationService.Create(comm)
	if err != nil {
		m.logger.Error("Failed to create communication", zap.Error(err))
		return fmt.Errorf("failed to create communication: %w", err)
	}

	m.logger.Info("Successfully processed Microsoft abuse report",
		zap.String("case_ref", caseModel.ReferenceNumber),
		zap.Uint("case_id", caseModel.ID))

	return nil
}

// ExtractSubjectURLs extracts URLs that might be subjects of the report
func (m *MicrosoftTemplate) ExtractSubjectURLs(email *letters.Email) []string {
	// First use the general URL extractor
	urls := m.contentExtractor.ExtractURLs(email)

	// Then look for Microsoft-specific patterns
	content := email.Text
	if content == "" {
		content = email.HTML
	}

	// Look for URLs in specific contexts
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()

		// Check for Microsoft's reporting patterns
		if strings.Contains(line, "Reported URL:") ||
			strings.Contains(line, "Content URL:") {
			// Extract URLs from this line specifically
			urlRegex := regexp.MustCompile(`https?:\/\/(www\.)?[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-zA-Z0-9()]{1,6}\b([-a-zA-Z0-9()@:%_\+.~#?&//=]*)`)
			matches := urlRegex.FindAllString(line, -1)

			for _, url := range matches {
				// Check if already in list
				found := false
				for _, existingURL := range urls {
					if existingURL == url {
						found = true
						break
					}
				}

				if !found {
					urls = append(urls, url)
				}
			}
		}
	}

	return urls
}

// ExtractReporter extracts reporter information from the Microsoft abuse report
func (m *MicrosoftTemplate) ExtractReporter(email *letters.Email) (string, string, error) {
	// Default values for Microsoft
	reporterEmail := "abuse@microsoft.com"
	reporterName := "Microsoft Abuse Team"

	// Try to find a better email if available
	if len(email.Headers.From) > 0 {
		fromEmail := email.Headers.From[0].Address
		fromName := email.Headers.From[0].Name

		if fromEmail != "" {
			reporterEmail = fromEmail
		}

		if fromName != "" {
			reporterName = fromName
		}
	}

	return reporterEmail, reporterName, nil
}

// ExtractCategory extracts the abuse category from a Microsoft abuse report
func (m *MicrosoftTemplate) ExtractCategory(email *letters.Email) models.CaseType {
	content := email.Text
	if content == "" {
		content = email.HTML
	}

	// Check for category indicators in content
	if strings.Contains(content, "phishing") ||
		strings.Contains(content, "fraud") {
		return models.CaseTypeOther
	}

	if strings.Contains(content, "spam") ||
		strings.Contains(content, "unwanted email") ||
		strings.Contains(content, "bulk") {
		return models.CaseTypeSpam
	}

	if strings.Contains(content, "copyright") ||
		strings.Contains(content, "DMCA") ||
		strings.Contains(content, "infringement") {
		return models.CaseTypeCopyrightViolation
	}

	if strings.Contains(content, "harassment") ||
		strings.Contains(content, "bullying") ||
		strings.Contains(content, "threatening") {
		return models.CaseTypeHarassment
	}

	// Default to Other if no specific category detected
	return models.CaseTypeOther
}
