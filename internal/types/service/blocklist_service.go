package service

import (
	"context"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal/core"
	coreModels "go.lumeweb.com/portal/db/models"
	"go.lumeweb.com/queryutil"
	"time"
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
		filters []queryutil.Filter,
		sorts []queryutil.Sort,
		pagination queryutil.Pagination,
	) ([]models.BlockList, int64, error)

	// GetBlocksByCaseID gets all blocked content associated with a case
	GetBlocksByCaseID(caseID uint) ([]models.BlockList, error)

	// BatchBlockFromScanResults creates block entries from scan results
	// Returns the number of blocks created
	BatchBlockFromScanResults(
		ctx context.Context, scanResults []*coreModels.ScanResult, defaultReason string, defaultSeverity string, defaultAction string, source string) (int, error)

	// GetBlocklistMetrics gets blocklist metrics within a date range
	GetBlocklistMetrics(start, end time.Time) (*BlocklistAnalytics, error)
}
