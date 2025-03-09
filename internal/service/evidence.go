package service

import (
	"context"
	"fmt"
	"go.lumeweb.com/portal-plugin-abuse/internal"
	"go.lumeweb.com/portal-plugin-abuse/internal/db"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/queryutil"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"io"
	"strings"
	"time"
	"unicode"
)

const abuseBucketName = "abuse"

// EvidenceServiceDefault implements the EvidenceService interface
type EvidenceServiceDefault struct {
	BaseService
	storageService core.StorageService
	caseSvc        typesSvc.CaseService
	reporterSvc    typesSvc.ReporterService
	emailSvc       typesSvc.EmailService
}

// Ensure EvidenceServiceDefault implements the interface
var _ typesSvc.EvidenceService = (*EvidenceServiceDefault)(nil)

// NewEvidenceService creates a new evidence service
func NewEvidenceService() (core.Service, []core.ContextBuilderOption, error) {
	svc := &EvidenceServiceDefault{}

	options := []core.ContextBuilderOption{
		func(ctx core.Context) (core.Context, error) {
			svc.BaseService.InitializeBaseService(ctx, svc)

			// Get the storage service from context
			storageService := core.GetService[core.StorageService](ctx, core.STORAGE_SERVICE)
			if storageService == nil {
				return ctx, fmt.Errorf("storage service not available")
			}
			svc.storageService = storageService

			// Get required services
			caseSvc := core.GetService[typesSvc.CaseService](ctx, typesSvc.CASE_SERVICE)
			if caseSvc == nil {
				return ctx, fmt.Errorf("case service not available")
			}
			svc.caseSvc = caseSvc

			reporterSvc := core.GetService[typesSvc.ReporterService](ctx, typesSvc.REPORTER_SERVICE)
			if reporterSvc == nil {
				return ctx, fmt.Errorf("reporter service not available")
			}
			svc.reporterSvc = reporterSvc

			emailSvc := core.GetService[typesSvc.EmailService](ctx, typesSvc.EMAIL_SERVICE)
			if emailSvc == nil {
				return ctx, fmt.Errorf("email service not available")
			}
			svc.emailSvc = emailSvc

			return ctx, nil
		},
	}

	return svc, options, nil
}

// ID returns the service identifier
func (s *EvidenceServiceDefault) ID() string {
	return typesSvc.EVIDENCE_SERVICE
}

// CreateFromData creates an evidence record from raw file data
func (s *EvidenceServiceDefault) CreateFromData(r io.ReadCloser, model *models.Evidence) (*models.Evidence, error) {
	// First create the evidence record to get the ID
	if err := db.Create(context.Background(), s.ctx, s.db, model); err != nil {
		s.logger.Error("Failed to create evidence record", zap.Error(err))
		return nil, fmt.Errorf("failed to create evidence record: %w", err)
	}

	// Generate structured object path
	objectKey := fmt.Sprintf("cases/%d/evidence/%d/%s",
		model.CaseID,
		model.ID,
		sanitizeFileName(model.FileName),
	)

	// Store the data using the structured path
	err := s.storageService.S3MultipartUpload(
		s.ctx.GetContext(),
		r,
		abuseBucketName,
		objectKey,
		uint64(model.FileSize),
	)

	if err != nil {
		// Rollback the DB record if storage fails
		s.db.Delete(model)
		s.logger.Error("Failed to store evidence data",
			zap.Error(err),
			zap.String("objectKey", objectKey))
		return nil, fmt.Errorf("failed to store evidence data: %w", err)
	}

	// Update the model with storage location
	model.StoragePath = objectKey
	if err := db.Update(context.Background(), s.ctx, s.db, model); err != nil {
		s.logger.Error("Failed to update evidence record with storage path",
			zap.Error(err),
			zap.String("objectKey", objectKey))
		return nil, fmt.Errorf("failed to update evidence record: %w", err)
	}

	go s.notifyEvidenceAdded(model)
	return model, nil
}

func (s *EvidenceServiceDefault) notifyEvidenceAdded(evidence *models.Evidence) {
	caseModel, err := s.caseSvc.GetByID(evidence.CaseID)
	if err != nil {
		s.logger.Error("Failed to get case for evidence notification",
			zap.Uint("caseID", evidence.CaseID),
			zap.Error(err))
		return
	}

	reporter, err := s.reporterSvc.GetByID(caseModel.ReporterID)
	if err != nil {
		s.logger.Error("Failed to get reporter for evidence notification",
			zap.Uint("reporterID", caseModel.ReporterID),
			zap.Error(err))
		return
	}

	siteURL := core.GetService[core.HTTPService](s.ctx, core.HTTP_SERVICE).APISubdomain(internal.PLUGIN_NAME, true)

	err = s.emailSvc.SendTemplatedEmail(
		[]string{reporter.Email},
		"case_evidence_added",
		core.MailerTemplateData{
			"CaseID":       caseModel.ReferenceNumber,
			"ReporterName": reporter.Name,
			"PortalName":   s.ctx.Config().Config().Core.PortalName,
			"FileName":     evidence.FileName,
			"FileSize":     fmt.Sprintf("%.2f MB", float64(evidence.FileSize)/1024/1024),
			"UploadDate":   evidence.CreatedAt.Format("January 2, 2006 15:04"),
			"CaseURL":      fmt.Sprintf("%s/case/%s", siteURL, caseModel.ReferenceNumber),
			"CaseType":     string(caseModel.Type),
			"CaseStatus":   string(caseModel.Status),
		},
	)

	if err != nil {
		s.logger.Error("Failed to send evidence notification",
			zap.Uint("caseID", evidence.CaseID),
			zap.Uint("evidenceID", evidence.ID),
			zap.Error(err))
	}
}

// CreateFromHash creates an evidence record using a pre-stored hash
func (s *EvidenceServiceDefault) CreateFromHash(
	caseID uint,
	submitterID uint,
	storagePath string,
	fileName string,
	contentType string,
	fileSize int64,
	source models.EvidenceSource,
	description string,
	metadata datatypes.JSON,
) (*models.Evidence, error) {
	// Create the evidence record
	evidence := &models.Evidence{
		CaseID:      caseID,
		SubmitterID: submitterID,
		FileName:    fileName,
		ContentType: contentType,
		StoragePath: storagePath,
		FileSize:    fileSize,
		Source:      source,
		Description: description,
		Metadata:    metadata,
	}

	if err := db.Create(context.Background(), s.ctx, s.db, evidence); err != nil {
		s.logger.Error("Failed to create evidence record", zap.Error(err), zap.String("storagePath", storagePath))
		return nil, fmt.Errorf("failed to create evidence record: %w", err)
	}

	return evidence, nil
}

// GetByID retrieves an evidence record by its ID
func (s *EvidenceServiceDefault) GetByID(id uint) (*models.Evidence, error) {
	var evidence models.Evidence
	if err := db.GetByID(context.Background(), s.ctx, s.db, id, &evidence); err != nil {
		s.logger.Error("Failed to get evidence by ID", zap.Error(err), zap.Uint("evidenceID", id))
		return nil, fmt.Errorf("failed to get evidence: %w", err)
	}
	return &evidence, nil
}

// List returns a list of evidence records with filtering, sorting and pagination
func (s *EvidenceServiceDefault) List(filters []queryutil.Filter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.Evidence, int64, error) {
	var evidence []models.Evidence
	var total int64

	if err := db.List(context.Background(), s.ctx, s.db, filters, sorts, pagination, &evidence, &total); err != nil {
		s.logger.Error("Failed to list evidence", zap.Error(err))
		return nil, 0, fmt.Errorf("failed to list evidence: %w", err)
	}

	return evidence, total, nil
}

// Update updates an existing evidence record
func (s *EvidenceServiceDefault) Update(evidence *models.Evidence) error {
	if err := db.Update(context.Background(), s.ctx, s.db, evidence); err != nil {
		s.logger.Error("Failed to update evidence", zap.Error(err), zap.Uint("evidenceID", evidence.ID))
		return fmt.Errorf("failed to update evidence: %w", err)
	}

	return nil
}

// Delete deletes an evidence record
func (s *EvidenceServiceDefault) Delete(id uint) error {
	var evidence models.Evidence
	if err := db.Delete(context.Background(), s.ctx, s.db, id, &evidence); err != nil {
		s.logger.Error("Failed to delete evidence", zap.Error(err), zap.Uint("evidenceID", id))
		return fmt.Errorf("failed to delete evidence: %w", err)
	}

	return nil
}

func (s *EvidenceServiceDefault) GetEvidenceMetrics(start time.Time, end time.Time) (*typesSvc.EvidenceAnalytics, error) {
	analytics := &typesSvc.EvidenceAnalytics{
		FilesPerCase: make(map[uint]int64),
	}

	// Get files per case
	var counts []struct {
		CaseID uint
		Count  int64
	}

	query := s.db.Model(&models.Evidence{})
	if !start.IsZero() {
		query = query.Where("created_at >= ?", start)
	}
	if !end.IsZero() {
		query = query.Where("created_at <= ?", end)
	}

	err := query.Select("case_id, count(*) as count").
		Group("case_id").
		Scan(&counts).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get files per case: %w", err)
	}

	var totalFiles int64
	var caseCount int64
	for _, c := range counts {
		analytics.FilesPerCase[c.CaseID] = c.Count
		totalFiles += c.Count
		caseCount++
	}

	if caseCount > 0 {
		analytics.AvgFilesPerCase = float64(totalFiles) / float64(caseCount)
	}

	return analytics, nil
}

func (s *EvidenceServiceDefault) GetByCaseID(caseID uint, pagination queryutil.Pagination) ([]models.Evidence, int64, error) {
	var evidenceList []models.Evidence
	var total int64

	filters := []queryutil.Filter{{Field: "case_id", Operator: queryutil.OperatorEquals, Value: caseID}}
	sorts := []queryutil.Sort{{Field: "created_at", Order: queryutil.OrderDesc}}

	if err := db.List(context.Background(), s.ctx, s.db, filters, sorts, pagination, &evidenceList, &total); err != nil {
		s.logger.Error("Failed to list evidence", zap.Error(err), zap.Uint("caseID", caseID))
		return nil, 0, fmt.Errorf("failed to list evidence: %w", err)
	}

	return evidenceList, total, nil
}

// GetContent retrieves the evidence content as a readable stream
func (s *EvidenceServiceDefault) GetContent(id uint) (io.ReadCloser, string, error) {
	// Get evidence record
	evidence, err := s.GetByID(id)
	if err != nil {
		return nil, "", err
	}

	// Get storage path
	uploadID := evidence.StoragePath

	// Retrieve data from storage
	reader, err := s.storageService.S3GetTemporaryUpload(s.ctx.GetContext(), nil, uploadID)
	if err != nil {
		s.logger.Error("Failed to retrieve evidence content", zap.Error(err), zap.String("uploadID", uploadID))
		return nil, "", fmt.Errorf("failed to retrieve evidence content: %w", err)
	}
	defer func(reader io.ReadCloser) {
		err := reader.Close()
		if err != nil {
			s.logger.Error("Failed to close evidence content", zap.Error(err), zap.String("uploadID", uploadID))
		}
	}(reader)

	return reader, evidence.ContentType, nil
}

func sanitizeFileName(name string) string {
	// Remove special characters and spaces
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '.' {
			return r
		}
		return '_'
	}, name)
}
