package email

import (
	"bytes"
	"context"
	"fmt"
	"github.com/mnako/letters"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"io"
	"sync"
	"time"

	"go.lumeweb.com/portal-plugin-abuse/internal/config"
	"go.lumeweb.com/portal/core"
	"go.uber.org/zap"
)

// EmailPipeline defines the interface for the email processing pipeline
type Pipeline interface {
	// ProcessEmail processes an email through the pipeline and returns structured results
	ProcessEmail(ctx context.Context, data io.Reader) (*ProcessingResult, error)

	// GetMetrics returns the current pipeline metrics
	GetMetrics() map[string]interface{}

	// Start initializes and starts the pipeline with the given email processor
	Start(processor Processor) error

	// Stop shuts down the pipeline
	Stop() error

	// SetConfigCallback sets a function to get the current email config
	SetConfigCallback(cb func() *config.EmailConfig)
}

// EmailProcessor defines a function type for processing emails
type Processor func(ctx context.Context, data io.Reader) error

// PipelineMetrics tracks metrics for the email pipeline
type PipelineMetrics struct {
	totalReceived        int64
	totalProcessed       int64
	totalErrors          int64
	totalARF             int64
	totalTemplateMatches int64
	totalNew             int64
	totalThreaded        int64
	processingTimes      []time.Duration
	mutex                sync.Mutex
}

// ProcessingResult contains the outcome of email processing
type ProcessingResult struct {
	ARFData     *ARFReport
	Case        *models.Case
	ThreadMatch *ThreadMatch
	IsARF       bool
	ProviderID  string
}

// PipelineDefault is the default implementation of the Pipeline interface
type PipelineDefault struct {
	ctx            core.Context
	logger         *core.Logger
	getConfig      func() *config.EmailConfig
	emailClient    IMAPClient
	arfProcessor   *ARFProcessor
	classifier     Classifier
	threadDetector *ThreadDetector
	templateProc   TemplateProcessor
	processor      Processor
	lock           sync.Mutex
	started        bool
	stopped        bool
	metrics        *PipelineMetrics
}

// SetConfigCallback sets the function to get current email config
func (p *PipelineDefault) SetConfigCallback(cb func() *config.EmailConfig) {
	p.getConfig = cb
}

// NewPipeline creates a new email pipeline with required dependencies
func NewPipeline(
	ctx core.Context,
	arfProcessor *ARFProcessor,
	classifier Classifier,
	threadDetector *ThreadDetector,
	templateProc TemplateProcessor,
) *PipelineDefault {
	return &PipelineDefault{
		ctx:    ctx,
		logger: ctx.Logger(),
		metrics: &PipelineMetrics{
			processingTimes: make([]time.Duration, 0, 100),
		},
		arfProcessor:   arfProcessor,
		classifier:     classifier,
		threadDetector: threadDetector,
		templateProc:   templateProc,
	}
}

// Start initializes and starts the pipeline
func (p *PipelineDefault) Start(processor Processor) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.started {
		return nil
	}

	// Store the email processor
	p.processor = processor

	// Initialize email receiving capability
	cfg := p.getConfig()
	if cfg != nil && cfg.ReceiveEnabled {
		// Check if IMAP settings are configured
		if cfg.IMAPHost != "" && cfg.IMAPUser != "" {
			// Use IMAP client for fetching emails
			p.emailClient = NewIMAPClient(p.ctx, cfg)

			// Configure IMAP client handler to adapt to legacy interface
			p.emailClient.SetEmailHandler(func(ctx context.Context, data io.Reader) error {
				_, err := p.ProcessEmail(ctx, data)
				return err
			})

			if err := p.emailClient.Start(); err != nil {
				p.logger.Error("Failed to start IMAP client", zap.Error(err))
				return fmt.Errorf("failed to start IMAP client: %w", err)
			}
			p.logger.Info("IMAP client started",
				zap.String("host", cfg.IMAPHost),
				zap.Int("port", cfg.IMAPPort),
				zap.String("mailbox", cfg.IMAPMailbox),
				zap.Int("poll_interval_seconds", cfg.PollInterval))
		} else {
			p.logger.Info("Email receiving is disabled")
		}
	} else {
		p.logger.Info("Email receiving is disabled")
	}

	p.started = true
	return nil
}

// Stop shuts down the pipeline
func (p *PipelineDefault) Stop() error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.stopped {
		return nil
	}

	// Stop IMAP client if it's running
	if p.emailClient != nil {
		if err := p.emailClient.Stop(); err != nil {
			p.logger.Error("Error stopping IMAP client", zap.Error(err))
		}
	}

	p.stopped = true
	return nil
}

// ProcessEmail processes an email through the pipeline and returns structured results
func (p *PipelineDefault) ProcessEmail(ctx context.Context, data io.Reader) (*ProcessingResult, error) {
	startTime := time.Now()
	defer func() {
		p.metrics.mutex.Lock()
		p.metrics.processingTimes = append(p.metrics.processingTimes, time.Since(startTime))
		if len(p.metrics.processingTimes) > 1000 {
			p.metrics.processingTimes = p.metrics.processingTimes[len(p.metrics.processingTimes)-1000:]
		}
		p.metrics.mutex.Unlock()
	}()

	// Track metrics
	p.metrics.mutex.Lock()
	p.metrics.totalReceived++
	p.metrics.mutex.Unlock()

	// Try ARF processing first
	isARF, buf := p.arfProcessor.IsARF(data)
	if isARF {
		arfReport, err := p.arfProcessor.Process(ctx, buf)
		if err != nil {
			p.recordError()
			return nil, err
		}
		p.metrics.mutex.Lock()
		p.metrics.totalARF++
		p.metrics.totalProcessed++
		p.metrics.mutex.Unlock()
		return &ProcessingResult{
			ARFData: arfReport,
			IsARF:   true,
		}, nil
	}

	// Parse regular email using the buffered data
	email, rawBytes, err := parseEmailWithRaw(buf)
	if err != nil {
		p.recordError()
		return nil, fmt.Errorf("failed to parse email: %w", err)
	}

	// Try provider template match
	if providerID, _, ok := p.templateProc.DetectProvider(bytes.NewReader(rawBytes)); ok {
		// Convert the email to a reader for processing
		emailReader := bytes.NewReader(rawBytes)
		if err := p.templateProc.Process(ctx, emailReader, providerID); err != nil {
			p.recordError()
			return nil, err
		}
		p.metrics.mutex.Lock()
		p.metrics.totalTemplateMatches++
		p.metrics.totalProcessed++
		p.metrics.mutex.Unlock()
		return &ProcessingResult{ProviderID: providerID}, nil
	}
	// Get reporter ID from email headers
	var reporterID uint
	if len(email.Headers.From) > 0 {
		// In real implementation we would look up the reporter ID from the email address
		// For now just use 0 as a placeholder
		reporterID = 0
	}

	// Detect existing thread using actual reporter ID
	threadMatch, _ := p.threadDetector.DetectThread(email, reporterID)

	// Create new case if no thread match
	if threadMatch == nil {
		// ClassifyEmail content only when needed
		classify := p.classifier.ClassifyEmail(email)
		// Get first 200 characters of email text as description
		description := email.Text
		if len(description) > 200 {
			description = description[:200] + "..."
		}

		_case := &models.Case{
			Type:        classify.CaseType,
			Status:      models.CaseStatusNew,
			Priority:    classify.Priority,
			Description: fmt.Sprintf("Email Subject: %s\nContent: %s", email.Headers.Subject, description),
			Source:      models.ReportSourceEmail,
		}
		p.metrics.mutex.Lock()
		p.metrics.totalNew++
		p.metrics.totalProcessed++
		p.metrics.mutex.Unlock()
		return &ProcessingResult{Case: _case}, nil
	}

	p.metrics.mutex.Lock()
	p.metrics.totalThreaded++
	p.metrics.totalProcessed++
	p.metrics.mutex.Unlock()
	return &ProcessingResult{ThreadMatch: threadMatch}, nil
}

// recordError records an error metric
func (p *PipelineDefault) recordError() {
	p.metrics.mutex.Lock()
	defer p.metrics.mutex.Unlock()
	p.metrics.totalErrors++
}

// GetMetrics returns the current metrics
// parseEmailWithRaw parses the email and returns both the parsed object and raw bytes
func parseEmailWithRaw(data io.Reader) (*letters.Email, []byte, error) {
	var buf bytes.Buffer
	tee := io.TeeReader(data, &buf)
	_email, err := letters.ParseEmail(tee)
	if err != nil {
		return nil, nil, err
	}
	return &_email, buf.Bytes(), nil
}

func (p *PipelineDefault) GetMetrics() map[string]interface{} {
	p.metrics.mutex.Lock()
	defer p.metrics.mutex.Unlock()

	// Calculate average processing time
	var avgProcessingTime time.Duration
	if len(p.metrics.processingTimes) > 0 {
		var total time.Duration
		for _, t := range p.metrics.processingTimes {
			total += t
		}
		avgProcessingTime = total / time.Duration(len(p.metrics.processingTimes))
	}

	return map[string]interface{}{
		"total_received":         p.metrics.totalReceived,
		"total_processed":        p.metrics.totalProcessed,
		"total_errors":           p.metrics.totalErrors,
		"total_arf":              p.metrics.totalARF,
		"total_template_matches": p.metrics.totalTemplateMatches,
		"total_new":              p.metrics.totalNew,
		"total_threaded":         p.metrics.totalThreaded,
		"avg_processing_time_ms": avgProcessingTime.Milliseconds(),
	}
}
