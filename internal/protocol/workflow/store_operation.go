package workflow

import (
	"go.lumeweb.com/portal-plugin-template/internal/protocol/handlers"
	"go.lumeweb.com/portal/core"
)

func NewStoreOperationHandler(protocol core.Protocol, ctx core.Context) core.OperationHandler {
	return handlers.NewStoreHandler(protocol, ctx)
}
