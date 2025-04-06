package service

import (
	"context"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal/core"
	coreModels "go.lumeweb.com/portal/db/models"
	"go.lumeweb.com/queryutil"
)

// BLOCKLIST_SERVICE is the service identifier for the blocklist service
const BLOCKLIST_SERVICE = "abuse.blocklist"

// BlockListService defines operations on the content block list
type BlockListService interface {
	core.Service

	// IsBlocked checks if content with the given hash is blocked
	// Returns (isBlocked, blockInfo, error)
	IsBlocked(hash core.StorageHash) (bool, *models.BlockList, error)

	// BlockContent adds a content hash to the block list
	BlockContent(block *models.BlockList) (*models.BlockList, error)

	// UnblockContent removes a content hash from the block list
	UnblockContent(hash core.StorageHash) error

	// GetBlockedContent gets a specific blocked content record by hash
	GetBlockedContent(hash core.StorageHash) (*models.BlockList, error)

	// ListBlockedContent lists all blocked content with filtering and pagination
	ListBlockedContent(
		filters []queryutil.CrudFilter,
		sorts []queryutil.Sort,
		pagination queryutil.Pagination,
	) ([]models.BlockList, int64, error)

	// GetBlocksByCaseID gets all blocked content associated with a case
	GetBlocksByCaseID(caseID uint) ([]models.BlockList, error)

	// BatchBlockFromScanResults creates block entries from scan results
	// Returns the number of blocks created
	BatchBlockFromScanResults(ctx context.Context, scanResults []*coreModels.ScanResult, defaultReason models.BlockReason, defaultSeverity models.BlockSeverity, defaultAction models.BlockAction, source models.BlockSource) (int, error)

	// GetBlocklistMetrics gets blocklist metrics with optional filters
	GetBlocklistMetrics(filters []queryutil.CrudFilter) (*BlocklistAnalytics, error)

	// GetBlockReasonCounts retrieves block reason counts within a time range.
	GetBlockReasonCounts(filters []queryutil.CrudFilter) ([]BlockReasonCount, error)
}

// BlockReasonCount represents a single block reason count.
type BlockReasonCount struct {
	BlockDate   string
	BlockReason string // String representation of BlockReason
	BlockCount  int64
}
