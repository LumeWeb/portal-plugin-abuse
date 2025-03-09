package scanner

import (
	"bytes"
	"context"
	"errors"
	"github.com/baruwa-enterprise/clamd"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multicodec"
	multihash "github.com/multiformats/go-multihash/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mocks2 "go.lumeweb.com/portal-plugin-abuse/internal/pkg/scanner/mocks"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	coreMocks "go.lumeweb.com/portal/core/testing/mocks"
	"go.lumeweb.com/portal/db/models"
	"io"
	"os"
	"testing"
	"time"
)

const testCIDStr = "QmSnuWmxptJZdLJpKRarxBMS2Ju2oANVrgbr2xWbie9b2D"

var testCID = mustParseStorageHash(testCIDStr)

func mustParseStorageHash(s string) core.StorageHash {
	h, err := core.ParseStorageHash(s)
	if err != nil {
		panic(err)
	}
	return h
}

func createIPLDHash(t *testing.T, content []byte) core.StorageHash {
	prefix := cid.Prefix{
		Version:  1,
		Codec:    uint64(multicodec.Raw),
		MhType:   multihash.SHA2_256,
		MhLength: -1,
	}
	_cid, err := prefix.Sum(content)
	require.NoError(t, err, "Failed to create CID prefix")

	block, err := blocks.NewBlockWithCid(content, _cid)
	require.NoError(t, err, "Failed to create IPLD block")

	hash, err := core.ParseStorageHash(block.Cid().String())
	require.NoError(t, err, "Invalid CID from IPFS")

	return hash
}

func TestClamScanner(t *testing.T) {
	ctx := coreTesting.NewTestContext(t)

	// Helper functions
	setupMocks := func(t *testing.T) (*coreMocks.MockStorageService, *coreMocks.MockUploadService, *mocks2.MockCoreStorageProtocolComposite) {
		coreTesting.ResetAllState()
		protocol := mocks2.NewMockCoreStorageProtocolComposite(t)
		protocol.EXPECT().Name().Return("ipfs").Maybe()
		core.RegisterProtocol("ipfs", protocol)
		return coreMocks.NewMockStorageService(t), coreMocks.NewMockUploadService(t), protocol
	}

	newTestScanner := func(mockStorage *coreMocks.MockStorageService, mockUpload *coreMocks.MockUploadService, client ScannerClient) *ClamScanner {
		return NewClamScannerWithClient(ctx, mockStorage, mockUpload, client, 1)
	}

	t.Run("ID", func(t *testing.T) {
		mockStorage, mockUpload, _ := setupMocks(t)
		s := newTestScanner(mockStorage, mockUpload, nil)
		assert.Equal(t, "clamav", s.ID())
	})

	t.Run("Name", func(t *testing.T) {
		mockStorage, mockUpload, _ := setupMocks(t)
		s := newTestScanner(mockStorage, mockUpload, nil)
		assert.Equal(t, "ClamAV Antivirus", s.Name())
	})

	t.Run("Priority", func(t *testing.T) {
		mockStorage, mockUpload, _ := setupMocks(t)
		s := newTestScanner(mockStorage, mockUpload, nil)
		assert.Equal(t, 50, s.Priority())
	})

	t.Run("TestConnection", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			mockStorage, mockUpload, _ := setupMocks(t)
			mockClient := mocks2.NewMockScannerClient(t)
			mockClient.EXPECT().Ping(mock.Anything).Return(true, nil)
			s := newTestScanner(mockStorage, mockUpload, mockClient)
			assert.NoError(t, s.TestConnection())
			mockClient.AssertExpectations(t)
		})

		t.Run("connection error", func(t *testing.T) {
			mockStorage, mockUpload, _ := setupMocks(t)
			mockClient := mocks2.NewMockScannerClient(t)
			mockClient.EXPECT().Ping(mock.Anything).Return(false, errors.New("connection failed"))
			s := newTestScanner(mockStorage, mockUpload, mockClient)
			assert.ErrorContains(t, s.TestConnection(), "connection failed")
			mockClient.AssertExpectations(t)
		})

		t.Run("not initialized", func(t *testing.T) {
			mockStorage := coreMocks.NewMockStorageService(t)
			mockUpload := coreMocks.NewMockUploadService(t)
			s := NewClamScannerWithClient(ctx, mockStorage, mockUpload, nil, 1)
			assert.ErrorContains(t, s.TestConnection(), "not initialized")
		})
	})

	t.Run("ScanContent", func(t *testing.T) {
		testData := []byte("test content")
		testCases := []struct {
			name           string
			mockResponses  []*clamd.Response
			mockError      error
			expectedPass   bool
			expectedReason string
		}{
			{
				name:           "clean content",
				mockResponses:  []*clamd.Response{{Status: "OK"}},
				expectedPass:   true,
				expectedReason: "No threats detected",
			},
			{
				name:           "virus detected",
				mockResponses:  []*clamd.Response{{Status: "FOUND", Signature: "TestSig"}},
				expectedPass:   false,
				expectedReason: "TestSig",
			},
			{
				name: "multiple threats",
				mockResponses: []*clamd.Response{
					{Status: "OK"},
					{Status: "FOUND", Signature: "TestSig"},
				},
				expectedPass:   false,
				expectedReason: "TestSig",
			},
			{
				name:           "scan error",
				mockError:      errors.New("scan failed"),
				expectedPass:   false,
				expectedReason: "Scan failed",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				coreTesting.ResetAllState()
				mockClient := mocks2.NewMockScannerClient(t)
				mockClient.EXPECT().ScanReader(mock.Anything, mock.Anything).Return(tc.mockResponses, tc.mockError)

				// Setup protocol mock
				protocol := mocks2.NewMockCoreStorageProtocolComposite(t)
				protocol.EXPECT().Name().Return("ipfs").Maybe()
				core.RegisterProtocol("ipfs", protocol)

				mockStorage := coreMocks.NewMockStorageService(t)
				mockUpload := coreMocks.NewMockUploadService(t)
				s := NewClamScannerWithClient(ctx, mockStorage, mockUpload, mockClient, 1)
				hash := testCID
				mockUpload.EXPECT().GetUpload(mock.Anything, hash).Return(&models.Upload{Protocol: "ipfs"}, nil)
				mockStorage.EXPECT().DownloadObject(
					mock.Anything,
					protocol,
					hash,
					mock.Anything,
				).Return(io.NopCloser(bytes.NewReader(testData)), nil)
				result, err := s.ScanContent(context.Background(), hash)

				if tc.mockError != nil {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}

				assert.Equal(t, tc.expectedPass, result.Passed)
				assert.Contains(t, result.Reason, tc.expectedReason)
				assert.WithinDuration(t, time.Now(), result.Timestamp, 5*time.Second)
				mockClient.AssertExpectations(t)
			})
		}

		t.Run("not connected", func(t *testing.T) {
			mockStorage := coreMocks.NewMockStorageService(t)
			mockUpload := coreMocks.NewMockUploadService(t)
			hash := testCID
			mockUpload.EXPECT().GetUpload(mock.Anything, hash).Return(nil, errors.New("not found"))
			s := NewClamScannerWithClient(ctx, mockStorage, mockUpload, nil, 1)
			result, err := s.ScanContent(context.Background(), hash)
			require.Error(t, err)
			assert.False(t, result.Passed)
		})
	})

	t.Run("WorkerPool", func(t *testing.T) {
		t.Run("concurrent scans", func(t *testing.T) {
			const numScans = 5
			mockClient := mocks2.NewMockScannerClient(t)
			mockClient.EXPECT().ScanReader(mock.Anything, mock.Anything).Times(numScans).Return([]*clamd.Response{
				{Status: "OK"},
			}, nil)

			mockStorage := coreMocks.NewMockStorageService(t)
			mockUpload := coreMocks.NewMockUploadService(t)

			// Setup mocks before launching goroutines
			hash := testCID
			mockUpload.EXPECT().GetUpload(mock.Anything, hash).Return(&models.Upload{Protocol: "ipfs"}, nil).Times(numScans)
			mockStorage.EXPECT().DownloadObject(mock.Anything, mock.Anything, hash, mock.Anything).Return(io.NopCloser(bytes.NewReader([]byte("data"))), nil).Times(numScans)

			s := NewClamScannerWithClient(ctx, mockStorage, mockUpload, mockClient, 2)
			results := make(chan *core.ScanResult, numScans)

			for i := 0; i < numScans; i++ {
				go func() {
					res, _ := s.ScanContent(context.Background(), hash)
					results <- res
				}()
			}

			for i := 0; i < numScans; i++ {
				res := <-results
				assert.True(t, res.Passed)
			}
			mockClient.AssertExpectations(t)
		})

		t.Run("queue saturation", func(t *testing.T) {
			mockClient := mocks2.NewMockScannerClient(t)
			// Only expect 1 scan since second task never completes
			mockClient.EXPECT().ScanReader(mock.Anything, mock.Anything).Times(1).Return([]*clamd.Response{
				{Status: "OK"},
			}, nil)

			mockStorage := coreMocks.NewMockStorageService(t)
			mockUpload := coreMocks.NewMockUploadService(t)
			s := NewClamScannerWithClient(ctx, mockStorage, mockUpload, mockClient, 1)

			// Setup blocking mechanism
			block := make(chan struct{})
			started := make(chan struct{})
			done := make(chan struct{})

			// Submit blocking task first
			s.SubmitToWorkerPool(func() {
				close(started)
				<-block
			})

			// Wait for blocking task to start
			<-started

			// Setup mocks for scan task
			hash := testCID
			mockUpload.EXPECT().GetUpload(mock.Anything, hash).Return(&models.Upload{Protocol: "ipfs"}, nil)
			mockStorage.EXPECT().DownloadObject(mock.Anything, mock.Anything, hash, mock.Anything).Return(io.NopCloser(bytes.NewReader([]byte("data"))), nil)

			// Submit scan task
			go func() {
				_, _ = s.ScanContent(context.Background(), hash)
				close(done)
			}()

			// Verify task is queued
			assert.Eventually(t, func() bool {
				return s.WorkerPoolWaitingQueueSize() == 1
			}, 100*time.Millisecond, 10*time.Millisecond, "Task should be queued")

			// Release blocking task
			close(block)

			// Wait for completion
			select {
			case <-done:
			case <-time.After(100 * time.Millisecond):
				t.Fatal("Test timed out waiting for task completion")
			}
		})
	})

	t.Run("Stop", func(t *testing.T) {
		mockStorage, mockUpload, _ := setupMocks(t)
		s := newTestScanner(mockStorage, mockUpload, nil)
		s.Stop() // Should not panic
	})
}

func TestClamScanner_Integration(t *testing.T) {
	if os.Getenv("CLAM_TEST") != "1" {
		t.Skip("Set CLAM_TEST=1 to enable integration tests")
	}

	ctx := coreTesting.NewTestContext(t)

	t.Run("live connection", func(t *testing.T) {
		coreTesting.ResetAllState()
		mockStorage := coreMocks.NewMockStorageService(t)
		mockUpload := coreMocks.NewMockUploadService(t)
		mockUpload.EXPECT().GetUpload(mock.Anything, mock.Anything).Return(&models.Upload{Protocol: "ipfs"}, nil)
		mockStorage.EXPECT().DownloadObject(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(io.NopCloser(bytes.NewReader([]byte("test content"))), nil)
		s := NewClamScanner(ctx, mockStorage, mockUpload, "unix", "/var/run/clamav/clamd.sock", 1)
		require.NoError(t, s.TestConnection())

		t.Run("clean file", func(t *testing.T) {
			safeContent := []byte("safe content")
			hash := createIPLDHash(t, safeContent)

			// Update mocks with real protocol and hash
			mockUpload.EXPECT().GetUpload(mock.Anything, hash).Return(&models.Upload{
				Protocol: "ipfs",
				Hash:     hash.Multihash(),
			}, nil)

			mockStorage.EXPECT().DownloadObject(
				mock.Anything,
				core.GetProtocol("ipfs").(core.StorageProtocol),
				hash,
				mock.Anything,
			).Return(io.NopCloser(bytes.NewReader(safeContent)), nil)

			result, err := s.ScanContent(context.Background(), hash)
			require.NoError(t, err)
			assert.True(t, result.Passed)
		})

		t.Run("eicar test", func(t *testing.T) {
			eicar := []byte(`X5O!P%@AP[4\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*`)
			hash := createIPLDHash(t, eicar)

			// Update mocks with real protocol and hash
			mockUpload.EXPECT().GetUpload(mock.Anything, hash).Return(&models.Upload{
				Protocol: "ipfs",
				Hash:     hash.Multihash(),
			}, nil)

			mockStorage.EXPECT().DownloadObject(
				mock.Anything,
				core.GetProtocol("ipfs").(core.StorageProtocol),
				hash,
				mock.Anything,
			).Return(io.NopCloser(bytes.NewReader(eicar)), nil)

			result, err := s.ScanContent(context.Background(), hash)
			require.NoError(t, err)
			assert.False(t, result.Passed)
			assert.Contains(t, result.Reason, "FOUND")
		})
	})
}
