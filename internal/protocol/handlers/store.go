package handlers

import (
	"bytes"
	"context"
	"fmt"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/portal/db/models"
	"go.lumeweb.com/portal/service"
	"go.uber.org/zap"
	"io"
)

type StoreHandler struct {
	protocol core.Protocol
	ctx      core.Context
}

func NewStoreHandler(protocol core.Protocol, ctx core.Context) *StoreHandler {
	return &StoreHandler{
		protocol: protocol,
		ctx:      ctx,
	}
}

func (h *StoreHandler) ValidateRequest(_ context.Context, _ *models.Request) error {
	// Validate request data
	return nil
}

func (h *StoreHandler) Execute(ctx context.Context, req *models.Request) error {
	// Get storage service and logger
	storage := core.GetService[core.StorageService](h.ctx, core.STORAGE_SERVICE)
	logger := h.ctx.Logger()

	storageProtocol, ok := h.protocol.(core.StorageProtocol)
	if !ok {
		return fmt.Errorf("protocol does not implement StorageProtocol")
	}

	// The data is already in temporary S3 storage from the initial upload
	// Get it from there and convert to ReadSeeker
	readCloser, err := storage.S3GetTemporaryUpload(ctx, storageProtocol, fmt.Sprintf("%d", req.ID))
	if err != nil {
		return fmt.Errorf("failed to get temporary upload: %w", err)
	}
	defer func(readCloser io.ReadCloser) {
		err := readCloser.Close()
		if err != nil {
			logger.Error("failed to close temporary upload reader", zap.Error(err))
		}
	}(readCloser)

	// Read all data into memory to create a ReadSeeker
	data, err := io.ReadAll(readCloser)
	if err != nil {
		return fmt.Errorf("failed to read upload  %w", err)
	}
	reader := bytes.NewReader(data)

	// Create upload request for final storage
	uploadReq := service.NewStorageUploadRequest(
		core.StorageUploadWithProtocol(storageProtocol),
		core.StorageUploadWithData(reader),
		core.StorageUploadWithSize(req.Size),
		core.StorageUploadWithProof(core.NewStorageHashFromMultihashBytes(req.Hash, 0, nil)),
	)

	// Store in final location using storage service
	_, err = storage.UploadObject(ctx, uploadReq)
	if err != nil {
		return fmt.Errorf("failed to store object: %w", err)
	}

	// Cleanup temporary upload
	if err := storage.S3DeleteTemporaryUpload(ctx, storageProtocol, fmt.Sprintf("%d", req.ID)); err != nil {
		// Log but don't fail if cleanup fails
		logger.Error("failed to cleanup temporary upload", zap.Error(err))
	}

	return nil
}

func (h *StoreHandler) GetStatus(_ context.Context, _ *models.Request) (core.RequestStatus, error) {
	return core.RequestStatus{
		State:   "completed",
		Message: "Upload completed",
	}, nil
}

func (h *StoreHandler) Cleanup(_ context.Context, _ *models.Request) error {
	return nil
}
