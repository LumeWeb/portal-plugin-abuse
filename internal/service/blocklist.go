package service

import (
	"context"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"time"

	"go.lumeweb.com/portal-plugin-abuse/internal/db"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	coreModels "go.lumeweb.com/portal/db/models"
	"go.lumeweb.com/queryutil"
	"go.uber.org/zap"
)

// BlockListServiceDefault implements the BlockListService interface
type BlockListServiceDefault struct {
	BaseService
}

// Ensure BlockListServiceDefault implements the interface
var _ typesSvc.BlockListService = (*BlockListServiceDefault)(nil)

// NewBlockListService creates a new blocklist service
func NewBlockListService() (core.Service, []core.ContextBuilderOption, error) {
	svc := &BlockListServiceDefault{}

	options := []core.ContextBuilderOption{
		func(ctx core.Context) (core.Context, error) {
			svc.BaseService.InitializeBaseService(ctx, svc)

			return ctx, nil
		},
	}

	return svc, options, nil
}

// ID returns the service identifier
func (s *BlockListServiceDefault) ID() string {
	return typesSvc.BLOCKLIST_SERVICE
}

// IsBlocked checks if content with the given hash is blocked
func (s *BlockListServiceDefault) IsBlocked(hash core.StorageHash) (bool, *models.BlockList, error) {
	var blockedContent models.BlockList
	err := db.GetByProperty(context.Background(), s.ctx, s.db, "hash", hash.Multihash(), &blockedContent)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil, nil
		}
		s.logger.Error("Failed to check if content is blocked", zap.Error(err), zap.Stringer("hash", hash))
		return false, nil, fmt.Errorf("failed to check if content is blocked: %w", err)
	}

	// Check if the block has expired
	if blockedContent.ExpiresAt != nil && blockedContent.ExpiresAt.Before(time.Now()) {
		// Remove the expired block
		if err := s.UnblockContent(hash); err != nil {
			s.logger.Error("Failed to unblock expired content", zap.Error(err), zap.Stringer("hash", hash))
			return false, &blockedContent, fmt.Errorf("content is blocked but failed to unblock expired content: %w", err) // Return blocked for now, but log the error
		}
		return false, nil, nil // Not blocked anymore
	}

	return true, &blockedContent, nil
}

// BlockContent adds a content hash to the block list
func (s *BlockListServiceDefault) BlockContent(block *models.BlockList) (*models.BlockList, error) {
	if err := db.Create(context.Background(), s.ctx, s.db, block); err != nil {
		s.logger.Error("Failed to block content", zap.Error(err), zap.Stringer("hash", block.Hash))
		return nil, fmt.Errorf("failed to block content: %w", err)
	}

	s.logger.Info("Blocked content", zap.Stringer("hash", block.Hash))
	return block, nil
}

// UnblockContent removes a content hash from the block list
func (s *BlockListServiceDefault) UnblockContent(hash core.StorageHash) error {
	blockedContent := &models.BlockList{Hash: hash.Multihash()}

	err := db.Delete(context.Background(), s.ctx, s.db, blockedContent.ID, &blockedContent)
	if err != nil {
		s.logger.Error("Failed to unblock content", zap.Error(err), zap.Stringer("hash", hash))
		return fmt.Errorf("failed to unblock content: %w", err)
	}

	s.logger.Info("Unblocked content", zap.Stringer("hash", hash))
	return nil
}

// GetBlockedContent gets a specific blocked content record by hash
func (s *BlockListServiceDefault) GetBlockedContent(hash core.StorageHash) (*models.BlockList, error) {
	hashStr := hash.String()

	var blockedContent models.BlockList
	err := db.GetByProperty(context.Background(), s.ctx, s.db, "Hash", hashStr, &blockedContent)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("content not found in blocklist")
		}
		s.logger.Error("Failed to get blocked content", zap.Error(err), zap.String("hash", hashStr))
		return nil, fmt.Errorf("failed to get blocked content: %w", err)
	}

	return &blockedContent, nil
}

// ListBlockedContent lists all blocked content with filtering and pagination
func (s *BlockListServiceDefault) ListBlockedContent(
	filters []queryutil.Filter,
	sorts []queryutil.Sort,
	pagination queryutil.Pagination,
) ([]models.BlockList, int64, error) {
	var blockedContent []models.BlockList
	var total int64

	err := db.List(context.Background(), s.ctx, s.db, filters, sorts, pagination, &blockedContent, &total)
	if err != nil {
		s.logger.Error("Failed to list blocked content", zap.Error(err))
		return nil, 0, fmt.Errorf("failed to list blocked content: %w", err)
	}

	return blockedContent, total, nil
}

// GetBlocksByCaseID gets all blocked content associated with a case
func (s *BlockListServiceDefault) GetBlocksByCaseID(caseID uint) ([]models.BlockList, error) {
	var blockedContent []models.BlockList
	err := s.db.Where("case_id = ?", caseID).Find(&blockedContent).Error
	if err != nil {
		s.logger.Error("Failed to get blocked content by case ID", zap.Error(err), zap.Uint("caseID", caseID))
		return nil, fmt.Errorf("failed to get blocked content: %w", err)
	}

	return blockedContent, nil
}

// GetBlocklistAnalytics gets blocklist metrics
func (s *BlockListServiceDefault) GetBlocklistMetrics(start, end time.Time) (*typesSvc.BlocklistAnalytics, error) {
	analytics := &typesSvc.BlocklistAnalytics{
		BlocksByReason:   make(map[models.BlockReason]int64),
		BlocksBySeverity: make(map[models.BlockSeverity]int64),
	}

	// Base query with date filtering
	query := s.db.Model(&models.BlockList{})
	if !start.IsZero() {
		query = query.Where("created_at >= ?", start)
	}
	if !end.IsZero() {
		query = query.Where("created_at <= ?", end)
	}

	// Total blocks
	if err := query.Count(&analytics.TotalBlocks).Error; err != nil {
		return nil, fmt.Errorf("failed to get total blocks: %w", err)
	}

	// Blocks by reason
	var reasonCounts []struct {
		Reason models.BlockReason
		Count  int64
	}
	if err := query.Select("reason, count(*) as count").
		Group("reason").
		Scan(&reasonCounts).Error; err != nil {
		return nil, fmt.Errorf("failed to get blocks by reason: %w", err)
	}
	for _, rc := range reasonCounts {
		analytics.BlocksByReason[rc.Reason] = rc.Count
	}

	// Blocks by severity
	var severityCounts []struct {
		Severity models.BlockSeverity
		Count    int64
	}
	if err := query.Select("severity, count(*) as count").
		Group("severity").
		Scan(&severityCounts).Error; err != nil {
		return nil, fmt.Errorf("failed to get blocks by severity: %w", err)
	}
	for _, sc := range severityCounts {
		analytics.BlocksBySeverity[sc.Severity] = sc.Count
	}

	return analytics, nil
}

// BatchBlockFromScanResults creates block entries from scan results
func (s *BlockListServiceDefault) BatchBlockFromScanResults(
	ctx context.Context,
	scanResults []*coreModels.ScanResult,
	defaultReason string,
	defaultSeverity string,
	defaultAction string,
	source string,
) (int, error) {
	blockedCount := 0
	for _, result := range scanResults {
		if !result.Passed {
			// Block the content
			block := &models.BlockList{
				Hash:        core.NewStorageHashFromRawMultihash(result.Hash).Multihash(),
				Reason:      defaultReason,
				Severity:    defaultSeverity,
				Action:      defaultAction,
				Description: result.Reason,
				Source:      source,
				Metadata:    result.Metadata,
				// Explicitly set zero values for clarity
				MimeType:   "",
				FileName:   "",
				Size:       0,
				UploaderID: nil,
				CaseID:     nil,
				BlockedBy:  0,
				ExpiresAt:  nil,
			}
			_, err := s.BlockContent(block)
			if err != nil {
				s.logger.Error("Failed to block content from scan result", zap.Error(err), zap.Stringer("hash", result.Hash))
				continue // Skip this result
			}
			blockedCount++
		}
	}

	return blockedCount, nil
}
