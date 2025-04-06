package service

import (
	"context"
	coreModels "go.lumeweb.com/portal/db/models"
	"testing"
	"time"

	mh "github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/portal-plugin-abuse/internal/db"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"go.lumeweb.com/queryutil"
	"gorm.io/gorm"
)

func TestBlockListService_IsBlocked_NotBlocked(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		blockListService := core.GetService[typesSvc.BlockListService](ctx, typesSvc.BLOCKLIST_SERVICE)
		assert.NotNil(tb, blockListService)

		// Create a mock storage hash
		mh, err := mh.Encode([]byte("testhash"), mh.SHA2_256)
		require.NoError(tb, err)
		testHash := core.NewStorageHashFromRawMultihash(mh)

		// Act
		blocked, blockList, err := blockListService.IsBlocked(testHash)

		// Assert
		assert.NoError(tb, err)
		assert.False(tb, blocked)
		assert.Nil(tb, blockList)
	}, coreTesting.WithService(typesSvc.BLOCKLIST_SERVICE, NewBlockListService))
}

func TestBlockListService_IsBlocked_Blocked(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		blockListService := core.GetService[typesSvc.BlockListService](ctx, typesSvc.BLOCKLIST_SERVICE)
		assert.NotNil(tb, blockListService)

		// Create a mock storage hash
		mh, err := mh.Encode([]byte("testhash"), mh.SHA2_256)
		require.NoError(tb, err)
		testHash := core.NewStorageHashFromRawMultihash(mh)

		// Create a blocked content entry
		blockedContent := &models.BlockList{
			Hash:      testHash.Multihash(),
			Reason:    models.BlockReasonSpam,
			Severity:  "high",
			Action:    "reject",
			BlockedBy: 1,
			Source:    models.BlockSourceAdmin,
		}
		err = ctx.DB().Create(blockedContent).Error
		require.NoError(tb, err)

		// Act
		blocked, blockList, err := blockListService.IsBlocked(testHash)

		// Assert
		assert.NoError(tb, err)
		assert.True(tb, blocked)
		assert.NotNil(tb, blockList)
		assert.Equal(tb, testHash.Multihash(), blockList.Hash)
	}, coreTesting.WithService(typesSvc.BLOCKLIST_SERVICE, NewBlockListService))
}

func TestBlockListService_IsBlocked_ExpiredContent_UnblocksSuccessfully(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		blockListService := core.GetService[typesSvc.BlockListService](ctx, typesSvc.BLOCKLIST_SERVICE)
		assert.NotNil(tb, blockListService)

		// Create a mock storage hash
		mh, err := mh.Encode([]byte("testhash"), mh.SHA2_256)
		require.NoError(tb, err)
		testHash := core.NewStorageHashFromRawMultihash(mh)

		expiredContent := &models.BlockList{
			Hash:      testHash.Multihash(),
			Reason:    models.BlockReasonSpam,
			Severity:  "high",
			Action:    "reject",
			BlockedBy: 1,
			Source:    models.BlockSourceAdmin,
			ExpiresAt: &[]time.Time{time.Now().Add(-1 * time.Hour)}[0],
		}
		err = ctx.DB().Create(expiredContent).Error
		require.NoError(tb, err)

		// Act
		blocked, blockList, err := blockListService.IsBlocked(testHash)

		// Assert
		assert.NoError(tb, err)
		assert.False(tb, blocked)
		assert.Nil(tb, blockList)

		// Verify the block is removed from the database
		var retrievedBlock models.BlockList
		err = ctx.DB().Where("hash = ?", testHash.Multihash()).First(&retrievedBlock).Error
		assert.Error(tb, err)
		assert.ErrorIs(tb, err, gorm.ErrRecordNotFound)
	}, coreTesting.WithService(typesSvc.BLOCKLIST_SERVICE, NewBlockListService))
}

func TestBlockListService_IsBlocked_ExpiredContent_FailedUnblock_KeepsBlocked(t *testing.T) {
	t.Skip("until we have a real life case to test for unblock failures")

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		blockListService := core.GetService[typesSvc.BlockListService](ctx, typesSvc.BLOCKLIST_SERVICE)
		assert.NotNil(tb, blockListService)

		// Create a mock storage hash
		mh, err := mh.Encode([]byte("testhash"), mh.SHA2_256)
		require.NoError(tb, err)
		testHash := core.NewStorageHashFromRawMultihash(mh)

		// Create a block with invalid foreign key to force unblock failure
		invalidCaseID := uint(999999)
		expiredButStuck := &models.BlockList{
			Hash:      testHash.Multihash(),
			Reason:    models.BlockReasonSpam,
			Severity:  "high",
			Action:    "reject",
			BlockedBy: 1,
			Source:    models.BlockSourceAdmin,
			ExpiresAt: &[]time.Time{time.Now().Add(-1 * time.Hour)}[0],
			CaseID:    &invalidCaseID, // Will cause foreign key violation
		}

		err = ctx.DB().Create(expiredButStuck).Error
		require.NoError(tb, err)

		// Act
		blocked, blockList, err := blockListService.IsBlocked(testHash)

		// Verify behavior
		assert.Error(tb, err)                                  // Should get error from DB
		assert.True(tb, blocked)                               // Still blocked due to failure
		assert.NotNil(tb, blockList)                           // Returns the block record
		assert.Equal(tb, testHash.Multihash(), blockList.Hash) // Verify correct record

		// Verify the block still exists in DB
		var dbBlock models.BlockList
		err = ctx.DB().Where("hash = ?", testHash.Multihash()).First(&dbBlock).Error
		assert.NoError(tb, err)
	}, coreTesting.WithService(typesSvc.BLOCKLIST_SERVICE, NewBlockListService))
}

func TestBlockListService_BlockContent(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		blockListService := core.GetService[typesSvc.BlockListService](ctx, typesSvc.BLOCKLIST_SERVICE)
		assert.NotNil(tb, blockListService)

		// Create a mock storage hash
		mh, err := mh.Encode([]byte("testhash"), mh.SHA2_256)
		require.NoError(tb, err)
		testHash := core.NewStorageHashFromRawMultihash(mh)

		block := &models.BlockList{
			Hash:      testHash.Multihash(),
			Reason:    models.BlockReasonSpam,
			Severity:  models.BlockSeverityHigh,
			Action:    models.BlockActionReject,
			BlockedBy: 1,
			Source:    models.BlockSourceAdmin,
		}

		// Act
		createdBlock, err := blockListService.BlockContent(block)

		// Assert
		assert.NoError(tb, err)
		assert.NotNil(tb, createdBlock)
		assert.Equal(tb, testHash.Multihash(), createdBlock.Hash)

		// Verify the block exists in the database
		var retrievedBlock models.BlockList
		err = ctx.DB().Where("hash = ?", testHash.Multihash()).First(&retrievedBlock).Error
		assert.NoError(tb, err)
		assert.Equal(tb, testHash.Multihash(), retrievedBlock.Hash)
	}, coreTesting.WithService(typesSvc.BLOCKLIST_SERVICE, NewBlockListService))
}

func TestBlockListService_UnblockContent(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		blockListService := core.GetService[typesSvc.BlockListService](ctx, typesSvc.BLOCKLIST_SERVICE)
		assert.NotNil(tb, blockListService)

		// Create a mock storage hash
		mh, err := mh.Encode([]byte("testhash"), mh.SHA2_256)
		require.NoError(tb, err)
		testHash := core.NewStorageHashFromRawMultihash(mh)

		// Create a blocked content entry with all required fields
		blockedContent := &models.BlockList{
			Hash:      testHash.Multihash(),
			Reason:    models.BlockReasonSpam,
			Severity:  models.BlockSeverityHigh,
			Action:    models.BlockActionReject,
			BlockedBy: 1,
			Source:    models.BlockSourceAdmin,
		}
		err = ctx.DB().Create(blockedContent).Error
		require.NoError(tb, err)

		// Act
		err = blockListService.UnblockContent(testHash)

		// Assert
		assert.NoError(tb, err)

		// Verify the block is removed from the database
		var retrievedBlock models.BlockList
		err = ctx.DB().Where("hash = ?", testHash.Multihash()).First(&retrievedBlock).Error
		assert.Error(tb, err)
		assert.ErrorIs(tb, err, gorm.ErrRecordNotFound)
	}, coreTesting.WithService(typesSvc.BLOCKLIST_SERVICE, NewBlockListService))
}

func TestBlockListService_GetBlockedContent(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		blockListService := core.GetService[typesSvc.BlockListService](ctx, typesSvc.BLOCKLIST_SERVICE)
		assert.NotNil(tb, blockListService)

		// Create a mock storage hash
		mh, err := mh.Encode([]byte("testhash"), mh.SHA2_256)
		require.NoError(tb, err)
		testHash := core.NewStorageHashFromRawMultihash(mh)

		// Create a blocked content entry
		blockedContent := &models.BlockList{
			Hash:      testHash.Multihash(),
			Reason:    models.BlockReasonSpam,
			Severity:  models.BlockSeverityHigh,
			Action:    models.BlockActionReject,
			BlockedBy: 1,
			Source:    models.BlockSourceAdmin,
		}
		err = ctx.DB().Create(blockedContent).Error
		require.NoError(tb, err)

		// Act
		retrievedBlock, err := blockListService.GetBlockedContent(testHash)

		// Assert
		assert.NoError(tb, err)
		assert.NotNil(tb, retrievedBlock)
		assert.Equal(tb, testHash.Multihash(), retrievedBlock.Hash)
	}, coreTesting.WithService(typesSvc.BLOCKLIST_SERVICE, NewBlockListService))
}

func TestBlockListService_GetBlockedContent_NotFound(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		blockListService := core.GetService[typesSvc.BlockListService](ctx, typesSvc.BLOCKLIST_SERVICE)
		assert.NotNil(tb, blockListService)

		// Create a mock storage hash
		mh2, err := mh.Encode([]byte("testhash2"), mh.SHA2_256)
		require.NoError(tb, err)
		testHash2 := core.NewStorageHashFromRawMultihash(mh2)

		// Act
		retrievedBlock, err := blockListService.GetBlockedContent(testHash2)

		// Assert
		assert.Error(tb, err)
		assert.ErrorIs(tb, err, db.ErrRecordNotFound)
		assert.Nil(tb, retrievedBlock)
	}, coreTesting.WithService(typesSvc.BLOCKLIST_SERVICE, NewBlockListService))
}

func TestBlockListService_ListBlockedContent(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		blockListService := core.GetService[typesSvc.BlockListService](ctx, typesSvc.BLOCKLIST_SERVICE)
		assert.NotNil(tb, blockListService)

		// Create mock storage hashes
		mh1, err := mh.Encode([]byte("testhash1"), mh.SHA2_256)
		require.NoError(tb, err)
		testHash1 := core.NewStorageHashFromRawMultihash(mh1)

		mh2, err := mh.Encode([]byte("testhash2"), mh.SHA2_256)
		require.NoError(tb, err)
		testHash2 := core.NewStorageHashFromRawMultihash(mh2)

		// Create blocked content entries
		blockedContent1 := &models.BlockList{
			Hash:      testHash1.Multihash(),
			Reason:    models.BlockReasonSpam,
			Severity:  models.BlockSeverityHigh,
			Action:    models.BlockActionReject,
			BlockedBy: 1,
			Source:    models.BlockSourceAdmin,
		}
		err = ctx.DB().Create(blockedContent1).Error
		require.NoError(tb, err)

		blockedContent2 := &models.BlockList{
			Hash:      testHash2.Multihash(),
			Reason:    models.BlockReasonSpam,
			Severity:  models.BlockSeverityHigh,
			Action:    models.BlockActionReject,
			BlockedBy: 1,
			Source:    models.BlockSourceAdmin,
		}
		err = ctx.DB().Create(blockedContent2).Error
		require.NoError(tb, err)

		// Act
		filters := []queryutil.CrudFilter{}
		sorts := []queryutil.Sort{}
		pagination := queryutil.DefaultPagination
		retrievedBlocks, total, err := blockListService.ListBlockedContent(filters, sorts, pagination)

		// Assert
		assert.NoError(tb, err)
		assert.Equal(tb, int64(2), total)
		assert.Len(tb, retrievedBlocks, 2)
	}, coreTesting.WithService(typesSvc.BLOCKLIST_SERVICE, NewBlockListService))
}

func TestBlockListService_GetBlocksByCaseID(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		blockListService := core.GetService[typesSvc.BlockListService](ctx, typesSvc.BLOCKLIST_SERVICE)
		assert.NotNil(tb, blockListService)

		// Create mock storage hashes
		mh1, err := mh.Encode([]byte("testhash1"), mh.SHA2_256)
		require.NoError(tb, err)
		testHash1 := core.NewStorageHashFromRawMultihash(mh1)

		mh2, err := mh.Encode([]byte("testhash2"), mh.SHA2_256)
		require.NoError(tb, err)
		testHash2 := core.NewStorageHashFromRawMultihash(mh2)

		// Create a case
		testCase := &models.Case{
			Model: gorm.Model{
				ID: 1,
			},
			ReferenceNumber: "testref",
			Type:            models.CaseTypeSpam,
			Status:          models.CaseStatusNew,
			Priority:        models.CasePriorityMedium,
			Source:          models.ReportSourceWebForm,
			Description:     "Test spam report",
			ReporterID:      1,
			SubjectID:       1,
		}
		err = ctx.DB().Create(testCase).Error
		require.NoError(tb, err)

		// Create blocked content entries associated with the case
		blockedContent1 := &models.BlockList{
			Hash:      testHash1.Multihash(),
			Reason:    models.BlockReasonSpam,
			Severity:  models.BlockSeverityHigh,
			Action:    models.BlockActionReject,
			BlockedBy: 1,
			CaseID:    &testCase.ID,
			Source:    models.BlockSourceAdmin,
		}
		err = ctx.DB().Create(blockedContent1).Error
		require.NoError(tb, err)

		blockedContent2 := &models.BlockList{
			Hash:      testHash2.Multihash(),
			Reason:    models.BlockReasonSpam,
			Severity:  models.BlockSeverityHigh,
			Action:    models.BlockActionReject,
			BlockedBy: 1,
			CaseID:    &testCase.ID,
			Source:    models.BlockSourceAdmin,
		}
		err = ctx.DB().Create(blockedContent2).Error
		require.NoError(tb, err)

		// Create a blocked content entry not associated with the case
		mh3, err := mh.Encode([]byte("testhash3"), mh.SHA2_256)
		require.NoError(tb, err)
		testHash3 := core.NewStorageHashFromRawMultihash(mh3)

		blockedContent3 := &models.BlockList{
			Hash:      testHash3.Multihash(),
			Reason:    models.BlockReasonSpam,
			Severity:  models.BlockSeverityHigh,
			Action:    models.BlockActionReject,
			BlockedBy: 1,
			Source:    models.BlockSourceAdmin,
		}
		err = ctx.DB().Create(blockedContent3).Error
		require.NoError(tb, err)

		// Act
		retrievedBlocks, err := blockListService.GetBlocksByCaseID(testCase.ID)

		// Assert
		assert.NoError(tb, err)
		assert.Len(tb, retrievedBlocks, 2)

		// Verify that the retrieved blocks are associated with the case
		for _, block := range retrievedBlocks {
			assert.Equal(tb, testCase.ID, *block.CaseID)
		}
	}, coreTesting.WithService(typesSvc.BLOCKLIST_SERVICE, NewBlockListService))
}

func TestBlockListService_GetBlocklistMetrics(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		blockListService := core.GetService[typesSvc.BlockListService](ctx, typesSvc.BLOCKLIST_SERVICE)
		assert.NotNil(tb, blockListService)

		// Define test times
		now := time.Now()
		start := now.Add(-24 * time.Hour)
		end := now

		// Create blocked content entries with different reasons and severities
		// All should have CreatedAt within the test date range
		blockedContent1 := &models.BlockList{
			Hash:      []byte("testhash1"),
			Reason:    "malware",
			Severity:  "critical",
			Action:    "reject",
			BlockedBy: 1,
			Source:    models.BlockSourceAdmin,
			Model: gorm.Model{
				CreatedAt: now,
			},
		}
		err := ctx.DB().Create(blockedContent1).Error
		require.NoError(tb, err)

		blockedContent2 := &models.BlockList{
			Hash:      []byte("testhash2"),
			Reason:    "spam",
			Severity:  "high",
			Action:    "reject",
			BlockedBy: 1,
			Source:    models.BlockSourceAdmin,
			Model: gorm.Model{
				CreatedAt: now,
			},
		}
		err = ctx.DB().Create(blockedContent2).Error
		require.NoError(tb, err)

		blockedContent3 := &models.BlockList{
			Hash:      []byte("testhash3"),
			Reason:    "malware",
			Severity:  "high",
			Action:    "reject",
			BlockedBy: 1,
			Source:    models.BlockSourceAdmin,
			Model: gorm.Model{
				CreatedAt: now,
			},
		}
		err = ctx.DB().Create(blockedContent3).Error
		require.NoError(tb, err)

		// Create filters for the date range
		filters := []queryutil.CrudFilter{
			queryutil.FieldGte("created_at", start),
			queryutil.FieldLte("created_at", end),
		}

		// Act
		analytics, err := blockListService.GetBlocklistMetrics(filters)

		// Assert
		assert.NoError(tb, err)
		assert.NotNil(tb, analytics)
		assert.Equal(tb, int64(3), analytics.TotalBlocks)
		assert.Equal(tb, int64(2), analytics.BlocksByReason["malware"])
		assert.Equal(tb, int64(1), analytics.BlocksByReason["spam"])
		assert.Equal(tb, int64(1), analytics.BlocksBySeverity["critical"])
		assert.Equal(tb, int64(2), analytics.BlocksBySeverity["high"])
	}, coreTesting.WithService(typesSvc.BLOCKLIST_SERVICE, NewBlockListService))
}

func TestBlockListService_BatchBlockFromScanResults(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		blockListService := core.GetService[typesSvc.BlockListService](ctx, typesSvc.BLOCKLIST_SERVICE)
		assert.NotNil(tb, blockListService)

		// Mock Scan Results
		mh1, err := mh.Encode([]byte("testhash1"), mh.SHA2_256)
		require.NoError(tb, err)

		mh2, err := mh.Encode([]byte("testhash2"), mh.SHA2_256)
		require.NoError(tb, err)

		scanResults := []*coreModels.ScanResult{
			{
				Hash:   mh1,
				Passed: false,
				Reason: "Malware detected",
			},
			{
				Hash:   mh2,
				Passed: true,
				Reason: "Clean",
			},
		}

		// Act
		blockedCount, err := blockListService.BatchBlockFromScanResults(context.Background(), scanResults, "malware", "critical", "reject", models.BlockSourceScanner)

		// Assert
		assert.NoError(tb, err)
		assert.Equal(tb, 1, blockedCount)

		// Verify the blocked content exists in the database
		var retrievedBlock models.BlockList
		err = ctx.DB().Where("hash = ?", mh1).First(&retrievedBlock).Error
		assert.NoError(tb, err)
		assert.Equal(tb, mh.Multihash(mh1), retrievedBlock.Hash)
	}, coreTesting.WithService(typesSvc.BLOCKLIST_SERVICE, NewBlockListService))
}
