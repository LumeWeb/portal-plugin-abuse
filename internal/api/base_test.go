package api

import (
	"go.lumeweb.com/portal-plugin-abuse/internal"
	"go.lumeweb.com/portal-plugin-abuse/internal/service/mocks"
	"go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"testing"
)

func TestMain(m *testing.M) {
	coreTesting.RunTests(m, coreTesting.TestMainOpts{
		CustomSetup: func() {
			coreTesting.AddGlobalTestContextOptions(
				coreTesting.WithConfig("core.domain", "example.com"),
				coreTesting.WithAPI(internal.PLUGIN_NAME, NewAbuseAPI),
				coreTesting.WithAPIExtension(NewAdminExtension()),
				coreTesting.WithMockServiceFactory(service.ABUSE_REPORT_SERVICE, mocks.NewMockAbuseReportService),
				coreTesting.WithMockServiceFactory(service.EMAIL_SERVICE, mocks.NewMockEmailService),
				coreTesting.WithMockServiceFactory(service.CASE_SERVICE, mocks.NewMockCaseService),
				coreTesting.WithMockServiceFactory(service.COMMUNICATION_SERVICE, mocks.NewMockCommunicationService),
				coreTesting.WithMockServiceFactory(service.SCAN_SERVICE, mocks.NewMockScanService),
				coreTesting.WithMockServiceFactory(service.TOKEN_SERVICE, mocks.NewMockTokenService),
				coreTesting.WithMockServiceFactory(service.REPORTER_SERVICE, mocks.NewMockReporterService),
				coreTesting.WithMockServiceFactory(service.EVIDENCE_SERVICE, mocks.NewMockEvidenceService),
				coreTesting.WithMockServiceFactory(service.BLOCKLIST_SERVICE, mocks.NewMockBlockListService),
				coreTesting.WithMockServiceFactory(service.SEARCH_SERVICE, mocks.NewMockSearchService),
				coreTesting.WithMockServiceFactory(service.SUBJECT_SERVICE, mocks.NewMockSubjectService),
			)
		},
	})
}
