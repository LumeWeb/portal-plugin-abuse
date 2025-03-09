package handlers

import (
	"context"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/portal/db/models"
)

type ScanHandler struct {
	protocol core.Protocol
	ctx     core.Context
}

func NewScanHandler(protocol core.Protocol, ctx core.Context) *ScanHandler {
	return &ScanHandler{
		protocol: protocol,
		ctx:      ctx,
	}
}

func (h *ScanHandler) ValidateRequest(_ context.Context, _ *models.Request) error {
	return nil
}

func (h *ScanHandler) Execute(_ context.Context, _ *models.Request) error {
	// Implement content scanning logic
	return nil
}

func (h *ScanHandler) GetStatus(_ context.Context, _ *models.Request) (core.RequestStatus, error) {
	return core.RequestStatus{
		State:   "completed",
		Message: "Scan completed",
	}, nil
}

func (h *ScanHandler) Cleanup(_ context.Context, _ *models.Request) error {
	return nil
}
