package workflow

import (
	"fmt"
	"go.lumeweb.com/portal-plugin-template/internal"
	"go.lumeweb.com/portal/core"
)

const (
	WorkflowUpload = "template.upload"
)

// RegisterWorkflows registers all workflows for the template protocol
func RegisterWorkflows(coordinator core.WorkflowCoordinator, protocol core.Protocol, ctx core.Context) error {
	// Register the upload workflow using core operation helpers
	err := coordinator.RegisterWorkflow(WorkflowUpload, []core.OperationStep{
		{
			Operation:       fmt.Sprintf("%s.store", internal.PLUGIN_NAME),
			Handler:         NewStoreOperationHandler(protocol, ctx),
			FailureBehavior: core.FailWorkflow,
		},
		{
			Operation:       fmt.Sprintf("%s.scan", internal.PLUGIN_NAME),
			Handler:         NewScanOperationHandler(protocol, ctx),
			FailureBehavior: core.ContinueWorkflow,
		},
	})

	return err
}
