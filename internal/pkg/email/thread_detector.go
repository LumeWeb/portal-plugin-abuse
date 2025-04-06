package email

import (
	"gorm.io/gorm"
	"regexp"
	"strings"
	"time"

	"github.com/hbollon/go-edlib"
	"github.com/mnako/letters"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/queryutil"
)

// Define default pagination settings for queries
var (
	defaultPagination = queryutil.Pagination{Start: 0, End: 10, PageSize: 10, Mode: "server"}
	largePagination   = queryutil.Pagination{Start: 0, End: 100, PageSize: 100, Mode: "server"}
)

// ThreadDetector helps detect and link related email threads
type ThreadDetector struct {
	ctx    core.Context
	logger *core.Logger
	db     *gorm.DB
}

// ThreadMatch represents a potential thread match
type ThreadMatch struct {
	Communication *models.Communication
	CaseID        uint
	Score         float64
	MatchType     string
}

// NewThreadDetector creates a new thread detector
func NewThreadDetector(ctx core.Context) *ThreadDetector {
	return &ThreadDetector{
		ctx:    ctx,
		logger: ctx.NamedLogger("thread-detector"),
		db:     ctx.DB(),
	}
}

// DetectThread attempts to find related threads for an email
func (t *ThreadDetector) DetectThread(email *letters.Email, reporterID uint) (*ThreadMatch, error) {
	// Check for nil email
	if email == nil {
		return nil, nil
	}

	// Try different methods to detect threading

	// 1. Check standard email headers (In-Reply-To, References)
	if match := t.checkStandardHeaders(email); match != nil {
		return match, nil
	}

	// 2. Check for case reference numbers in subject
	if match := t.checkCaseReference(email); match != nil {
		return match, nil
	}

	// 3. Check for similar subjects
	if match := t.checkSubjectSimilarity(email, reporterID); match != nil {
		return match, nil
	}

	// 4. Check recent communications from same sender
	if match := t.checkSenderHistory(email, reporterID); match != nil {
		return match, nil
	}

	// No thread match found
	return nil, nil
}

// checkStandardHeaders checks for standard email threading headers
func (t *ThreadDetector) checkStandardHeaders(email *letters.Email) *ThreadMatch {
	// Get communication service
	communicationService := core.GetService[typesSvc.CommunicationService](t.ctx, typesSvc.COMMUNICATION_SERVICE)
	if communicationService == nil {
		t.logger.Warn("Communication service not available")
		return nil
	}

	// Check In-Reply-To headers
	if len(email.Headers.InReplyTo) > 0 {
		for _, inReplyTo := range email.Headers.InReplyTo {
			threadID := string(inReplyTo)
			comm, err := communicationService.GetByThreadID(threadID)
			if err == nil {
				return &ThreadMatch{
					Communication: comm,
					CaseID:        comm.CaseID,
					Score:         1.0,
					MatchType:     "in-reply-to",
				}
			}
		}
	}

	// Check References headers
	if len(email.Headers.References) > 0 {
		for _, reference := range email.Headers.References {
			threadID := string(reference)
			comm, err := communicationService.GetByThreadID(threadID)
			if err == nil {
				return &ThreadMatch{
					Communication: comm,
					CaseID:        comm.CaseID,
					Score:         0.9,
					MatchType:     "references",
				}
			}
		}
	}

	// Check Message-ID format for case reference
	if email.Headers.MessageID != "" {
		msgID := string(email.Headers.MessageID)
		// Look for case reference patterns in Message-ID
		if strings.Contains(msgID, "CASE-") {
			// Extract CASE-XXXXX pattern
			casePattern := regexp.MustCompile(`CASE-[A-Za-z0-9]+`)
			matches := casePattern.FindAllString(msgID, -1)

			if len(matches) > 0 {
				caseRef := matches[0]
				// Get case service
				caseService := core.GetService[typesSvc.CaseService](t.ctx, typesSvc.CASE_SERVICE)
				if caseService != nil {
					cases, total, err := caseService.Search(t.ctx, caseRef, nil, defaultPagination)
					if err == nil && total > 0 {
						// Get the most recent communication for this case
						comms, _, err := communicationService.ListByCaseID(cases[0].ID, nil, nil, defaultPagination)
						if err == nil && len(comms) > 0 {
							return &ThreadMatch{
								Communication: &comms[0],
								CaseID:        cases[0].ID,
								Score:         0.8,
								MatchType:     "message-id-case-ref",
							}
						}
					}
				}
			}
		}
	}

	return nil
}

// checkCaseReference checks for case references in the subject line
func (t *ThreadDetector) checkCaseReference(email *letters.Email) *ThreadMatch {
	subject := email.Headers.Subject

	// Look for case reference patterns (CASE-XXXXX)
	casePattern := regexp.MustCompile(`\b(CASE-[A-Za-z0-9]+)\b`)
	matches := casePattern.FindAllString(subject, -1)

	if len(matches) > 0 {
		caseRef := matches[0]

		// Get case service
		caseService := core.GetService[typesSvc.CaseService](t.ctx, typesSvc.CASE_SERVICE)
		if caseService == nil {
			t.logger.Warn("Case service not available")
			return nil
		}

		cases, total, err := caseService.Search(t.ctx, caseRef, nil, defaultPagination)
		if err == nil && total > 0 {
			// Get communication service
			communicationService := core.GetService[typesSvc.CommunicationService](t.ctx, typesSvc.COMMUNICATION_SERVICE)
			if communicationService == nil {
				t.logger.Warn("Communication service not available")
				return nil
			}

			// Get the most recent communication for this case
			filters := queryutil.Filters(
				queryutil.Or(
				queryutil.StringField("type").Eq(string(models.CommunicationTypeEmail)),
				queryutil.StringField("type").Eq(string(models.CommunicationTypeResponse)),
			))
			comms, _, err := communicationService.ListByCaseID(cases[0].ID, filters, nil, defaultPagination)
			if err == nil && len(comms) > 0 {
				return &ThreadMatch{
					Communication: &comms[0],
					CaseID:        cases[0].ID,
					Score:         0.95,
					MatchType:     "subject-case-ref",
				}
			}
		}
	}

	return nil
}

// checkSubjectSimilarity checks for similar subject lines
func (t *ThreadDetector) checkSubjectSimilarity(email *letters.Email, reporterID uint) *ThreadMatch {
	subject := email.Headers.Subject

	// Clean up the subject (remove Re:, Fwd:, etc.)
	subject = t.cleanSubject(subject)

	// Get cases for this reporter
	caseService := core.GetService[typesSvc.CaseService](t.ctx, typesSvc.CASE_SERVICE)
	if caseService == nil {
		t.logger.Warn("Case service not available")
		return nil
	}

	// Find cases from this reporter
	reporterFilter := queryutil.Filters(
		queryutil.NumberField[uint]("reporter_id").Eq(reporterID),
	)
	cases, total, err := caseService.List(reporterFilter, nil, largePagination)
	if err != nil || total == 0 || len(cases) == 0 {
		return nil
	}

	// Get communication service
	communicationService := core.GetService[typesSvc.CommunicationService](t.ctx, typesSvc.COMMUNICATION_SERVICE)
	if communicationService == nil {
		t.logger.Warn("Communication service not available")
		return nil
	}

	var bestMatch *ThreadMatch

	// Check each case
	for _, caseModel := range cases {
		// Get communications for this case
		filters := queryutil.Filters(queryutil.StringField("type").Eq(string(models.CommunicationTypeEmail)))
		comms, _, err := communicationService.ListByCaseID(caseModel.ID, filters, nil, defaultPagination)
		if err != nil || len(comms) == 0 {
			continue
		}

		// Check each communication's subject for similarity
		for _, comm := range comms {
			// Only check email communications with subjects
			if comm.Type != models.CommunicationTypeEmail {
				continue
			}

			// Try to extract subject from the communication content
			commSubject := t.extractSubjectFromContent(comm.Content)
			if commSubject == "" {
				continue
			}

			// Clean up the comparison subject
			commSubject = t.cleanSubject(commSubject)

			// Skip if either subject is too short
			if len(subject) < 5 || len(commSubject) < 5 {
				continue
			}

			// Calculate similarity using Jaro-Winkler distance
			similarity := edlib.JaroWinklerSimilarity(subject, commSubject)
			similarityF64 := float64(similarity) // Convert to float64

			// Update best match if this is better
			if similarity > 0.85 && (bestMatch == nil || similarityF64 > bestMatch.Score) {
				bestMatch = &ThreadMatch{
					Communication: &comm,
					CaseID:        caseModel.ID,
					Score:         similarityF64,
					MatchType:     "subject-similarity",
				}
			}
		}
	}

	return bestMatch
}

// checkSenderHistory checks for recent communications from the same sender
func (t *ThreadDetector) checkSenderHistory(email *letters.Email, reporterID uint) *ThreadMatch {
	// Get communication service
	communicationService := core.GetService[typesSvc.CommunicationService](t.ctx, typesSvc.COMMUNICATION_SERVICE)
	if communicationService == nil {
		t.logger.Warn("Communication service not available")
		return nil
	}

	// Search for recent communications from this reporter
	// This would be more efficient with a direct query, but we'll use the existing service
	var recentComms []models.Communication
	var recentCase uint

	// Get case service
	caseService := core.GetService[typesSvc.CaseService](t.ctx, typesSvc.CASE_SERVICE)
	if caseService == nil {
		t.logger.Warn("Case service not available")
		return nil
	}

	// Find cases from this reporter
	reporterFilter := queryutil.Filters(
		queryutil.NumberField[uint]("reporter_id").Eq(reporterID),
	)
	cases, total, err := caseService.List(reporterFilter, nil, largePagination)
	if err != nil || total == 0 || len(cases) == 0 {
		return nil
	}

	// Check recent communications in each case
	for _, caseModel := range cases {
		// Skip cases that are closed or resolved
		if caseModel.Status == models.CaseStatusClosed || caseModel.Status == models.CaseStatusResolved {
			continue
		}

		// Get communications for this case
		filters := []queryutil.CrudFilter{queryutil.Or(
			queryutil.StringField("type").Eq(string(models.CommunicationTypeEmail)),
			queryutil.StringField("type").Eq(string(models.CommunicationTypeResponse)),
		)}
		comms, _, err := communicationService.ListByCaseID(caseModel.ID, filters, nil, defaultPagination)
		if err != nil || len(comms) == 0 {
			continue
		}

		// Check if there's a recent communication (within the last 7 days)
		for _, comm := range comms {
			if comm.SenderID == reporterID {
				timeSince := time.Since(comm.CreatedAt)
				if timeSince.Hours() < 168 { // 7 days in hours
					recentComms = append(recentComms, comm)
					recentCase = caseModel.ID
					break
				}
			}
		}
	}

	// If we found recent communications, use the latest one
	if len(recentComms) > 0 {
		// Find the most recent communication
		var latestComm models.Communication
		latestComm = recentComms[0]

		for _, comm := range recentComms {
			if comm.CreatedAt.After(latestComm.CreatedAt) {
				latestComm = comm
			}
		}

		return &ThreadMatch{
			Communication: &latestComm,
			CaseID:        recentCase,
			Score:         0.7,
			MatchType:     "recent-sender",
		}
	}

	return nil
}

// cleanSubject removes common prefixes and normalizes a subject line
func (t *ThreadDetector) cleanSubject(subject string) string {
	// Remove Re:, Fwd:, etc.
	prefixPattern := regexp.MustCompile(`^(?i)(re|fwd|fw|forward|reply|response|re:\s*|fwd:\s*|fw:\s*|forward:\s*|reply:\s*|response:\s*)+\s*`)
	subject = prefixPattern.ReplaceAllString(subject, "")

	// Remove case reference numbers
	caseRefPattern := regexp.MustCompile(`\[CASE-[A-Za-z0-9]+\]|\(CASE-[A-Za-z0-9]+\)|\bCASE-[A-Za-z0-9]+\b`)
	subject = caseRefPattern.ReplaceAllString(subject, "")

	// Normalize whitespace
	whitespacePattern := regexp.MustCompile(`\s+`)
	subject = whitespacePattern.ReplaceAllString(subject, " ")

	// Trim leading/trailing whitespace
	subject = strings.TrimSpace(subject)

	return subject
}

// extractSubjectFromContent attempts to extract the subject from email content
func (t *ThreadDetector) extractSubjectFromContent(content string) string {
	// Check for the empty subject case first
	emptySubjectRegex := regexp.MustCompile(`(?m)^Subject:\s*$`)
	if emptySubjectRegex.MatchString(content) {
		return ""
	}

	// Common patterns for subject lines in email content
	patterns := []string{
		`(?m)^Subject:\s*(.+?)(?:\r?\n)`, // Multiline mode for ^ to match start of each line
		`Subject:\s*([^\r\n]+)`,          // Fallback for multiline content
	}

	for _, pattern := range patterns {
		regex := regexp.MustCompile(pattern)
		matches := regex.FindStringSubmatch(content)
		if len(matches) > 1 && matches[1] != "" {
			// We found a non-empty subject
			return matches[1]
		}
	}

	return ""
}
