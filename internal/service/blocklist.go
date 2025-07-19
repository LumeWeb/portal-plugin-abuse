package service

import (
	"context"
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
	return svc, core.ContextOptions(
		core.ContextWithStartupFunc(func(ctx core.Context) error {
			svc.BaseService.InitializeBaseService(ctx, svc)
			return nil
		}),
	), nil
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
		if db.IsRecordNotFound(err) {
			return false, nil, nil
		}
		s.logger.Error("Failed to check if content is blocked", zap.Error(err), zap.Stringer("hash", hash))
		return false, nil, fmt.Errorf("failed to check if content is blocked: %w", err)
	}

	// Check if the block has expired
	if blockedContent.ExpiresAt != nil && blockedContent.ExpiresAt.Before(time.Now()) {
		// Attempt to remove the expired block
		if err := s.UnblockContent(hash); err != nil {
			s.logger.Error("Failed to unblock expired content - keeping blocked state",
				zap.Error(err),
				zap.Stringer("hash", hash),
				zap.Time("expired_at", *blockedContent.ExpiresAt))
			return true, &blockedContent, err
		}
		// Return false/nil to indicate block was removed
		return false, nil, nil
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
	blockedContent := &models.BlockList{}

	err := db.DeleteByProperty(context.Background(), s.ctx, s.db, "hash", hash.Multihash(), &blockedContent)
	if err != nil {
		s.logger.Error("Failed to unblock content", zap.Error(err), zap.Stringer("hash", hash))
		return fmt.Errorf("failed to unblock content: %w", err)
	}

	s.logger.Info("Unblocked content", zap.Stringer("hash", hash))
	return nil
}

// GetBlockedContent gets a specific blocked content record by hash
func (s *BlockListServiceDefault) GetBlockedContent(hash core.StorageHash) (*models.BlockList, error) {
	var blockedContent models.BlockList
	err := db.GetByProperty(context.Background(), s.ctx, s.db, "hash", hash.Multihash(), &blockedContent)
	if err != nil {
		if db.IsRecordNotFound(err) {
			return nil, db.ErrRecordNotFound
		}
		s.logger.Error("Failed to get blocked content", zap.Error(err), zap.String("hash", hash.String()))
		return nil, db.HandleDBError(err, "GetBlockedContent", "BlockList", 0)
	}

	return &blockedContent, nil
}

// ListBlockedContent lists all blocked content with filtering and pagination
func (s *BlockListServiceDefault) ListBlockedContent(
	filters []queryutil.CrudFilter,
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
		return nil, db.HandleDBError(err, "GetBlockedContent", "BlockList", 0)
	}

	return blockedContent, nil
}

// GetBlocklistMetrics gets blocklist metrics with optional filters
func (s *BlockListServiceDefault) GetBlocklistMetrics(filters []queryutil.CrudFilter) (*typesSvc.BlocklistAnalytics, error) {
	analytics := &typesSvc.BlocklistAnalytics{
		BlocksByReason:   make(map[models.BlockReason]int64),
		BlocksBySeverity: make(map[models.BlockSeverity]int64),
	}

	// Get total blocks count
	total, err := db.Count[models.BlockList](context.Background(), s.ctx, s.db, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get total blocks: %w", err)
	}
	analytics.TotalBlocks = total

	// Get blocks by reason
	var reasonCounts []struct {
		Reason models.BlockReason
		Count  int64
	}
	err = db.ListAggregate(context.Background(), s.ctx, s.db, filters, nil, queryutil.Pagination{}, &reasonCounts,
		func(tx *gorm.DB) *gorm.DB {
			return tx.Model(&models.BlockList{}).Select("reason, count(*) as count").Group("reason")
		})
	if err != nil {
		return nil, fmt.Errorf("failed to get blocks by reason: %w", err)
	}
	for _, rc := range reasonCounts {
		analytics.BlocksByReason[rc.Reason] = rc.Count
	}

	// Get blocks by severity
	var severityCounts []struct {
		Severity models.BlockSeverity
		Count    int64
	}
	err = db.ListAggregate(context.Background(), s.ctx, s.db, filters, nil, queryutil.Pagination{}, &severityCounts,
		func(tx *gorm.DB) *gorm.DB {
			return tx.Model(&models.BlockList{}).Select("severity, count(*) as count").Group("severity")
		})
	if err != nil {
		return nil, fmt.Errorf("failed to get blocks by severity: %w", err)
	}
	for _, sc := range severityCounts {
		analytics.BlocksBySeverity[sc.Severity] = sc.Count
	}

	return analytics, nil
}

// BatchBlockFromScanResults creates block entries from scan results
func (s *BlockListServiceDefault) BatchBlockFromScanResults(ctx context.Context, scanResults []*coreModels.ScanResult, defaultReason models.BlockReason, defaultSeverity models.BlockSeverity, defaultAction models.BlockAction, source models.BlockSource) (int, error) {
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
				return 0, db.HandleDBError(err, "BatchBlockFromScanResults", "BlockList", 0)
			}
			blockedCount++
		}
	}

	return blockedCount, nil
}

// IsSubjectBlocked checks if a subject ID is blocked
func (s *BlockListServiceDefault) IsSubjectBlocked(subjectID uint) (bool, error) {
	if subjectID == 0 {
		return false, fmt.Errorf("invalid subjectID: cannot be zero")
	}

	count, err := db.Count[models.BlockList](context.Background(), s.ctx, s.db,
		queryutil.Filters(queryutil.FieldEqual("abuse_subjects.id", subjectID)),
		db.WithDBJoin("JOIN", "abuse_subjects", "abuse_blocklist.hash = abuse_subjects.identifier"))

	if err != nil {
		s.logger.Error("Failed to check subject block status",
			zap.Error(err),
			zap.Uint("subjectID", subjectID))
		return false, fmt.Errorf("failed to check subject block status: %w", err)
	}

	return count > 0, nil
}

// GetBlockReasonCounts retrieves block reason counts within a time range.
func (s *BlockListServiceDefault) GetBlockReasonCounts(filters []queryutil.CrudFilter) ([]typesSvc.BlockReasonCount, error) {
	var counts []models.BlockReasonCount

	var start, end time.Time

	gteFilter := queryutil.FindFilterWithOperator(filters, "block_date", queryutil.OpGte)
	if gteFilter != nil {
		if t, ok := gteFilter.GetValue().(time.Time); ok {
			start = t
		}
	}

	lteFilter := queryutil.FindFilterWithOperator(filters, "block_date", queryutil.OpLte)
	if lteFilter != nil {
		if t, ok := lteFilter.GetValue().(time.Time); ok {
			end = t
		}
	}

	if start.After(end) {
		return nil, fmt.Errorf("start date cannot be after end date")
	}

	query := s.db.Model(&models.BlockReasonCount{})

	if !start.IsZero() {
		query = query.Where("block_date >= ?", start)
	}
	if !end.IsZero() {
		query = query.Where("block_date <= ?", end)
	}

	err := query.Find(&counts).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get block reason counts: %w", err)
	}

	// Convert models.BlockReasonCount to typesSvc.BlockReasonCount
	result := make([]typesSvc.BlockReasonCount, len(counts))
	for i, count := range counts {
		result[i] = typesSvc.BlockReasonCount{
			BlockDate:   count.BlockDate,
			BlockReason: string(count.BlockReason), // Convert to string
			BlockCount:  count.BlockCount,
		}
	}

	return result, nil
}
