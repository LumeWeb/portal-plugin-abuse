package api

import (
	"github.com/stretchr/testify/mock"
	"testing"

	"go.lumeweb.com/portal-plugin-abuse/internal/service/mocks"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	coreMocks "go.lumeweb.com/portal/core/testing/mocks"
)

func setupAdminServices(t *testing.T) (coreTesting.TestContext, *AdminExtension, *coreMocks.MockAccessService) {
	ctx := coreTesting.NewTestContext(t)
	accessSvc := coreMocks.NewMockAccessService(t)

	// Setup access service expectations
	accessSvc.On("RegisterRoute", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	ctx.RegisterService(typesSvc.SEARCH_SERVICE, mocks.NewMockSearchService(t))
	ctx.RegisterService(typesSvc.EVIDENCE_SERVICE, mocks.NewMockEvidenceService(t))
	ctx.RegisterService(typesSvc.SUBJECT_SERVICE, mocks.NewMockSubjectService(t))
	ctx.RegisterService(typesSvc.BLOCKLIST_SERVICE, mocks.NewMockBlockListService(t))
	ctx.RegisterService(typesSvc.REPORTER_SERVICE, mocks.NewMockReporterService(t))
	ctx.RegisterService(typesSvc.CASE_SERVICE, mocks.NewMockCaseService(t))
	ctx.RegisterService(typesSvc.COMMUNICATION_SERVICE, mocks.NewMockCommunicationService(t))
	ctx.RegisterService(typesSvc.SCAN_SERVICE, mocks.NewMockScanService(t))
	ctx.RegisterService(core.ACCESS_SERVICE, accessSvc)

	// Create admin extension using factory
	factory := NewAdminExtension(ctx)

	apiExt, opts, _ := factory()

	for _, opt := range opts {
		newCtx, _ := opt(ctx)
		ctx = newCtx.(coreTesting.TestContext)
	}

	return ctx, apiExt.(*AdminExtension), accessSvc
}
