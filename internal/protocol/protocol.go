package protocol

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"go.lumeweb.com/portal-plugin-template/internal/protocol/request"
	"io"
	"strconv"
	"sync"
	"time"

	"go.lumeweb.com/portal-plugin-template/internal"
	pluginConfig "go.lumeweb.com/portal-plugin-template/internal/config"
	"go.lumeweb.com/portal-plugin-template/internal/protocol/handlers"
	"go.lumeweb.com/portal-plugin-template/internal/protocol/workflow"
	"go.lumeweb.com/portal-plugin-template/internal/service"
	"go.lumeweb.com/portal/config"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/portal/db/models"
	"go.uber.org/zap"
)

var (
	_ core.Protocol        = (*Protocol)(nil)
	_ core.ProtocolStart   = (*Protocol)(nil)
	_ core.ProtocolStop    = (*Protocol)(nil)
	_ core.StorageProtocol = (*Protocol)(nil)
)

type Protocol struct {
	portalConfig config.Manager
	config       *pluginConfig.Config
	logger       *core.Logger
	itemService  core.Service
	storage      core.StorageService
	coordinator  core.WorkflowCoordinator
	ctx          core.Context

	// Internal state
	uploads   map[string]*uploadState
	uploadsMu sync.RWMutex
	isRunning bool
}


// uploadState tracks an ongoing upload
type uploadState struct {
	ID        string
	Size      uint64
	Uploaded  uint64
	Started   time.Time
	Completed bool
	Hash      core.StorageHash
}

func (p *Protocol) Name() string {
	return internal.PLUGIN_NAME
}

func (p *Protocol) Config() config.ProtocolConfig {
	if p.config == nil {
		p.config = &pluginConfig.Config{}
	}
	return p.config
}

// Operations returns the list of operations supported by this protocol
func (p *Protocol) Operations() []core.Operation {
	return []core.Operation{
		core.NewStoreOperation(p.Name(), handlers.NewStoreHandler(p, p.ctx)),
		core.NewOperation(
			fmt.Sprintf("%s.scan", p.Name()),
			core.OpTypeScan,
			handlers.NewScanHandler(p, p.ctx),
		),
	}
}

func NewProtocol() (*Protocol, []core.ContextBuilderOption, error) {
	proto := &Protocol{
		uploads: make(map[string]*uploadState),
	}

	opts := core.ContextOptions(
		core.ContextWithStartupFunc(func(ctx core.Context) error {
			proto.ctx = ctx
			proto.portalConfig = ctx.Config()
			proto.logger = ctx.Logger()
			proto.storage = ctx.Service(core.STORAGE_SERVICE).(core.StorageService)
			proto.itemService = core.GetService[service.ItemService](ctx, service.ITEM_SERVICE)
			proto.coordinator = ctx.Service("workflow").(core.WorkflowCoordinator)

			// Load config
			cfg := proto.portalConfig.GetProtocol(internal.PLUGIN_NAME).(*pluginConfig.Config)
			proto.config = cfg

			// Get request service
			requestSvc := ctx.Service(core.REQUEST_SERVICE).(core.RequestService)

			// Register request model
			requestSvc.RegisterRequestModel(proto.Name(), &request.TemplateRequest{})

			// Register workflows
			if err := workflow.RegisterWorkflows(proto.coordinator, proto, ctx); err != nil {
				return fmt.Errorf("failed to register workflows: %w", err)
			}

			proto.logger.Info("Template protocol initialized",
				zap.String("storage_path", cfg.StoragePath),
				zap.Int("max_items", cfg.MaxItems),
				zap.Bool("cache_enabled", cfg.CacheEnabled))

			return nil
		}),
	)

	return proto, opts, nil
}

func (p *Protocol) Start(_ core.Context) error {
	p.logger.Info("Starting template protocol")
	p.isRunning = true
	return nil
}

func (p *Protocol) Stop(_ core.Context) error {
	p.logger.Info("Stopping template protocol")
	p.isRunning = false
	return nil
}

// StorageProtocol implementation
func (p *Protocol) EncodeFileName(hash core.StorageHash) string {
	return hash.Multihash().B58String()
}

func (p *Protocol) Hash(r io.Reader, _ uint64) (core.StorageHash, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return nil, err
	}
	return core.NewStorageHash(h.Sum(nil), uint64(sha256.Size), 0, nil), nil
}

func (p *Protocol) HandleUpload(ctx context.Context, reader io.Reader, size uint64) (core.StorageHash, error) {
	if !p.isRunning {
		return nil, errors.New("protocol not running")
	}

	// Calculate hash first
	h := sha256.New()
	if _, err := io.Copy(h, reader); err != nil {
		return nil, fmt.Errorf("failed to calculate hash: %w", err)
	}

	// Create storage hash
	hash := core.NewStorageHash(h.Sum(nil), uint64(sha256.Size), 0, nil)

	// Start upload workflow
	req := &models.Request{
		Protocol: p.Name(),
		Hash:     hash.Multihash(),
		Size:     size,
	}

	_, err := p.coordinator.StartWorkflow(ctx, workflow.WorkflowUpload, req)
	if err != nil {
		return nil, fmt.Errorf("failed to start upload workflow: %w", err)
	}

	// Track upload state
	state := &uploadState{
		ID:      fmt.Sprintf("%d", req.ID),
		Size:    size,
		Started: time.Now(),
		Hash:    hash,
	}

	p.uploadsMu.Lock()
	p.uploads[state.ID] = state
	p.uploadsMu.Unlock()

	return hash, nil
}

// GetUploadStatus gets the status of an upload from its workflow state
func (p *Protocol) GetUploadStatus(uploadID string) (*uploadState, error) {
	requestID, err := strconv.ParseUint(uploadID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid upload ID: %w", err)
	}

	status, err := p.coordinator.GetWorkflowStatus(context.Background(), uint(requestID))
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow status: %w", err)
	}

	// Get request service to fetch size from request data
	requestSvc := p.ctx.Service(core.REQUEST_SERVICE).(core.RequestService)
	req, err := requestSvc.GetRequest(context.Background(), uint(requestID))
	if err != nil {
		return nil, fmt.Errorf("failed to get request: %w", err)
	}

	return &uploadState{
		ID:        uploadID,
		Size:      req.Size,
		Started:   status.StartedAt,
		Completed: status.Status == string(models.RequestStatusCompleted),
		Hash:      core.NewStorageHashFromMultihashBytes(req.Hash, 0, nil),
	}, nil
}
