package service

import (
	"github.com/stretchr/testify/mock"
	"go.lumeweb.com/portal-plugin-abuse/internal"
	"go.lumeweb.com/portal-plugin-abuse/internal/config"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/migrations"
	"go.lumeweb.com/portal-plugin-abuse/internal/service/mocks"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	coreMocks "go.lumeweb.com/portal/core/testing/mocks"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	coreTesting.RunTests(m, coreTesting.TestMainOpts{
		WithDB:       true,
		DBMigrations: true,
		CustomSetup: func() {
			coreTesting.AddGlobalTestContextOptions(
				coreTesting.WithMockServiceFactory(typesSvc.CASE_SERVICE, mocks.NewMockCaseService),
				coreTesting.WithMockServiceFactory(typesSvc.REPORTER_SERVICE, mocks.NewMockReporterService),
				coreTesting.WithMockServiceFactory(typesSvc.SUBJECT_SERVICE, mocks.NewMockSubjectService),
				coreTesting.WithMockServiceFactory(typesSvc.EMAIL_SERVICE, mocks.NewMockEmailService),
				coreTesting.WithMockServiceFactory(typesSvc.TOKEN_SERVICE, mocks.NewMockTokenService),
				coreTesting.WithMockServiceFactory(typesSvc.COMMUNICATION_SERVICE, mocks.NewMockCommunicationService),
				coreTesting.WithMockServiceFactory(typesSvc.EVIDENCE_SERVICE, mocks.NewMockEvidenceService),
				coreTesting.WithMockServiceFactory(typesSvc.BLOCKLIST_SERVICE, mocks.NewMockBlockListService),
				func(ctx coreTesting.TestContext) (coreTesting.TestContext, error) {
					// Register a startup function that will create and register the mock later
					startupOpt := core.ContextWithStartupFunc(func(coreCtx core.Context) error {
						mockWorkflowService := core.GetService[*coreMocks.MockWorkflowService](ctx, core.WORKFLOW_SERVICE)
						mockWorkflowService.On("RegisterWorkflow", mock.Anything, mock.Anything).Return(nil).Maybe()

						mockContentScannerSvc := core.GetService[*coreMocks.MockContentScannerService](ctx, core.CONTENT_SCANNER_SERVICE)
						mockContentScannerSvc.On("RegisterScanner", mock.AnythingOfType("*scanner.ClamScanner")).Return(nil).Maybe()
						return nil
					})

					return coreTesting.ProcessCtxOptions(ctx, coreTesting.WrapCoreOption(startupOpt))
				},
				coreTesting.WithServiceConfig(typesSvc.EMAIL_SERVICE, &config.EmailConfig{}),
				coreTesting.WithServiceConfig(typesSvc.SCAN_SERVICE, &config.ScanConfig{}),
				coreTesting.WithSQLitePluginMigrations(internal.PLUGIN_NAME, migrations.GetSQLite()),
			)
		},
	})
}

// waitForAsync runs a function and waits for the done channel to receive
func waitForAsync(tb coreTesting.TB, done chan bool, fn func()) {
	go func() {
		if fn != nil {
			fn()
		}
	}()

	select {
	case <-done:
		return
	case <-time.After(1 * time.Second):
		tb.Fatal("Timeout waiting for async operation")
	}
}
