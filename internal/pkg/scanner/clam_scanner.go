package scanner

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/baruwa-enterprise/clamd"
	"github.com/gammazero/workerpool"
	"github.com/samber/lo"
	"go.lumeweb.com/portal/core"
	"go.uber.org/zap"
)

var _ core.ContentScanner = (*ClamScanner)(nil)
var _ ScannerClient = (*clamd.Client)(nil)

// ScannerClient defines the minimal ClamAV client interface needed
type ScannerClient interface {
	Ping(ctx context.Context) (bool, error)
	ScanReader(ctx context.Context, r io.Reader) ([]*clamd.Response, error)
}

// ClamScanner implements the core.ContentScanner interface for ClamAV
type ClamScanner struct {
	Ctx           core.Context
	logger        *core.Logger
	Network       string
	Address       string
	timeout       time.Duration
	connected     bool
	client        ScannerClient // Use interface instead of concrete type
	storage       core.StorageService
	uploadService core.UploadService
	workerPool    *workerpool.WorkerPool
}

// NewClamScanner creates a new ClamAV scanner with default client
func NewClamScanner(ctx core.Context, storage core.StorageService, uploadService core.UploadService, network, address string, maxWorkers int) *ClamScanner {
	scanner := &ClamScanner{
		Ctx:           ctx,
		logger:        ctx.NamedLogger("clam_scanner"),
		storage:       storage,
		uploadService: uploadService,
		Network:       network,
		Address:       address,
		timeout:       30 * time.Second,
		workerPool:    workerpool.New(maxWorkers),
	}

	// Create client directly
	client, err := clamd.NewClient(network, address)
	if err != nil {
		scanner.logger.Error("Failed to create ClamAV client", zap.Error(err))
		return scanner
	}

	scanner.client = client

	// Test connection
	if _, err := client.Ping(context.Background()); err != nil {
		scanner.logger.Error("Failed to connect to ClamAV daemon", zap.Error(err))
	} else {
		scanner.connected = true
		scanner.logger.Info("Connected to ClamAV",
			zap.String("network", network),
			zap.String("address", address))
	}

	return scanner
}

// NewClamScannerWithClient creates a scanner with a custom client (for testing)
func NewClamScannerWithClient(ctx core.Context, storage core.StorageService, uploadService core.UploadService, client ScannerClient, maxWorkers int) *ClamScanner {
	return &ClamScanner{
		Ctx:           ctx,
		logger:        ctx.NamedLogger("clam_scanner"),
		storage:       storage,
		uploadService: uploadService,
		client:        client,
		workerPool:    workerpool.New(maxWorkers),
		connected:     true, // Assume connected when using constructor with client
	}
}

// ID returns the scanner ID
func (s *ClamScanner) ID() string {
	return "clamav"
}

// Name returns the scanner name
func (s *ClamScanner) Name() string {
	return "ClamAV Antivirus"
}

// Description returns the scanner description
func (s *ClamScanner) Description() string {
	return "Scans content streams for viruses and malware using ClamAV"
}

// ScanContent implements core.ContentScanner interface
func (s *ClamScanner) ScanContent(ctx context.Context, hash core.StorageHash) (*core.ScanResult, error) {
	// Get upload record to determine protocol
	upload, err := s.uploadService.GetUpload(context.Background(), hash)
	if err != nil {
		return &core.ScanResult{
			ScannerID: s.ID(),
			Passed:    false,
			Reason:    fmt.Sprintf("Failed to find upload record: %v", err),
			Timestamp: time.Now(),
		}, fmt.Errorf("failed to find upload record: %w", err)
	}

	// Get content from storage using protocol and hash
	protocol := core.GetProtocol(upload.Protocol)
	storageProtocol, ok := protocol.(core.StorageProtocol)
	if !ok {
		return s.scanError(
			fmt.Sprintf("Invalid storage protocol: %T", protocol),
			fmt.Errorf("invalid storage protocol: %T", protocol),
		)
	}

	contentReader, err := s.storage.DownloadObject(context.Background(), storageProtocol, hash, 0)
	if err != nil {
		return s.scanError(
			fmt.Sprintf("Failed to retrieve content: %v", err),
			fmt.Errorf("failed to retrieve content: %w", err),
		)
	}
	defer func(contentReader io.ReadCloser) {
		err := contentReader.Close()
		if err != nil {
			s.logger.Error("Failed to close reader", zap.Error(err))
		}
	}(contentReader)

	// Create channels to receive the result and error from the worker
	resultChan := make(chan *core.ScanResult, 1)
	errorChan := make(chan error, 1)

	// Wrap scan request as a function for the worker pool
	scanFunc := func() {
		// Call your internal scan function
		scanData, err2 := s.scanData(ctx, contentReader, "application/octet-stream", hash.Multihash().String())
		if err2 != nil {
			s.logger.Error("Failed to scan data", zap.Error(err2))
			errorChan <- err2
			return
		}

		resultChan <- scanData
	}

	// Submit the scan function and wait for it to complete
	s.SubmitToWorkerPool(scanFunc)

	// Receive the result and error from the channels
	select {
	case result := <-resultChan:
		return result, nil
	case err := <-errorChan:
		return s.scanError("Scan failed", err)
	}
}

// scanError creates a standardized error response
func (s *ClamScanner) scanError(reason string, err error) (*core.ScanResult, error) {
	return &core.ScanResult{
		ScannerID: s.ID(),
		Passed:    false,
		Reason:    reason,
		Timestamp: time.Now(),
	}, err
}

// scanData internal implementation for scanning data from an io.Reader
func (s *ClamScanner) scanData(ctx context.Context, reader io.Reader, contentType, filename string) (*core.ScanResult, error) {
	if !s.connected || s.client == nil {
		return s.scanError("ClamAV scanner is not connected",
			fmt.Errorf("ClamAV scanner is not connected"))
	}

	// Set default filename if empty
	if filename == "" {
		filename = "unknown"
	}

	// Use our service to scan the stream
	responses, err := s.client.ScanReader(ctx, reader)
	if err != nil {
		s.logger.Error("ScanStream failed", zap.Error(err))
		return &core.ScanResult{
			ScannerID: s.ID(),
			Passed:    false,
			Reason:    fmt.Sprintf("Scan failed: %s", err),
			Timestamp: time.Now(),
		}, fmt.Errorf("scan failed: %w", err)
	}

	// Process the scan responses using lo
	found := lo.Filter(responses, func(response *clamd.Response, _ int) bool {
		return response.Status == "FOUND"
	})

	if threat := lo.FirstOr(found, nil); threat != nil {
		return &core.ScanResult{
			ScannerID: s.ID(),
			Passed:    false,
			Reason:    fmt.Sprintf("Virus detected: %s", threat.Signature),
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"signature":    threat.Signature,
				"filename":     threat.Filename,
				"threat_level": "high",
				"scanner_name": s.Name(),
			},
		}, nil
	}

	return &core.ScanResult{
		ScannerID: s.ID(),
		Passed:    true,
		Reason:    "No threats detected",
		Timestamp: time.Now(),
	}, nil
}

// Priority implements core.ContentScanner interface
func (s *ClamScanner) Priority() int {
	return 50 // Medium priority
}

// TestConnection verifies that the scanner can connect to the ClamAV daemon
func (s *ClamScanner) TestConnection() error {
	if s.client == nil {
		return fmt.Errorf("ClamAV client not initialized")
	}
	_, err := s.client.Ping(context.Background())
	return err
}

// Optionally, add a method to stop the worker pool when the service is shut down
func (s *ClamScanner) Stop() {
	s.workerPool.StopWait() // Wait for all tasks to complete
}

// WorkerPoolWaitingQueueSize exposes the waiting queue size for testing
func (s *ClamScanner) WorkerPoolWaitingQueueSize() int {
	return s.workerPool.WaitingQueueSize()
}

// SubmitToWorkerPool exposes worker pool submission for testing
func (s *ClamScanner) SubmitToWorkerPool(task func()) {
	s.workerPool.Submit(task)
}
