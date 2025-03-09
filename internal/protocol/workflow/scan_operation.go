package workflow

import (
	"go.lumeweb.com/portal-plugin-template/internal/protocol/handlers"
	"go.lumeweb.com/portal/core"
)

func NewScanOperationHandler(protocol core.Protocol, ctx core.Context) core.OperationHandler {
	return handlers.NewScanHandler(protocol, ctx)
}
