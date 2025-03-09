package api

import (
	"fmt"
	"github.com/gorilla/mux"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
)

// AdminExtension extends the Admin API with abuse management functionality
type AdminExtension struct {
	ctx              core.Context
	logger           *core.Logger
	searchService    typesSvc.SearchService
	evidenceService  typesSvc.EvidenceService
	subjectService   typesSvc.SubjectService
	blockListService typesSvc.BlockListService
	reporterService  typesSvc.ReporterService
	caseService      typesSvc.CaseService
	commService      typesSvc.CommunicationService
	scanService      typesSvc.ScanService
}

// NewAdminExtension creates a new Admin API extension for abuse management
func NewAdminExtension(ctx core.Context) core.APIExtensionFactory {
	return func() (core.APIExtension, []core.ContextBuilderOption, error) {
		ext := &AdminExtension{
			ctx:    ctx,
			logger: ctx.NamedLogger("abuse.admin_extension"),
		}

		// Get and verify all required services
		if ext.searchService = core.GetService[typesSvc.SearchService](ctx, typesSvc.SEARCH_SERVICE); ext.searchService == nil {
			return nil, nil, fmt.Errorf("search service not available")
		}
		if ext.evidenceService = core.GetService[typesSvc.EvidenceService](ctx, typesSvc.EVIDENCE_SERVICE); ext.evidenceService == nil {
			return nil, nil, fmt.Errorf("evidence service not available")
		}
		if ext.subjectService = core.GetService[typesSvc.SubjectService](ctx, typesSvc.SUBJECT_SERVICE); ext.subjectService == nil {
			return nil, nil, fmt.Errorf("subject service not available")
		}
		if ext.blockListService = core.GetService[typesSvc.BlockListService](ctx, typesSvc.BLOCKLIST_SERVICE); ext.blockListService == nil {
			return nil, nil, fmt.Errorf("blocklist service not available")
		}
		if ext.reporterService = core.GetService[typesSvc.ReporterService](ctx, typesSvc.REPORTER_SERVICE); ext.reporterService == nil {
			return nil, nil, fmt.Errorf("reporter service not available")
		}
		if ext.caseService = core.GetService[typesSvc.CaseService](ctx, typesSvc.CASE_SERVICE); ext.caseService == nil {
			return nil, nil, fmt.Errorf("case service not available")
		}
		if ext.commService = core.GetService[typesSvc.CommunicationService](ctx, typesSvc.COMMUNICATION_SERVICE); ext.commService == nil {
			return nil, nil, fmt.Errorf("communication service not available")
		}
		if ext.scanService = core.GetService[typesSvc.ScanService](ctx, typesSvc.SCAN_SERVICE); ext.scanService == nil {
			return nil, nil, fmt.Errorf("scan service not available")
		}

		return ext, core.ContextOptions(func(ctx core.Context) (core.Context, error) {
			return ctx, nil
		}), nil
	}
}

// TargetAPI returns the name of the API this extension targets
func (e *AdminExtension) TargetAPI() string {
	return "admin" // This extension targets the admin API
}

// Configure is called to set up routes on the admin API router
func (e *AdminExtension) Configure(router *mux.Router, accessSvc core.AccessService) error {
	// Create a subrouter for abuse management
	abuseRouter := router.PathPrefix("/abuse").Subrouter()

	// Register all the route handlers
	if err := e.registerCaseHandlers(abuseRouter, accessSvc); err != nil {
		return err
	}

	if err := e.registerReporterHandlers(abuseRouter, accessSvc); err != nil {
		return err
	}

	if err := e.registerEvidenceHandlers(abuseRouter, accessSvc); err != nil {
		return err
	}

	if err := e.registerSubjectHandlers(abuseRouter, accessSvc); err != nil {
		return err
	}

	if err := e.registerStatusUpdateHandlers(abuseRouter, accessSvc); err != nil {
		return err
	}

	if err := e.registerCommunicationHandlers(abuseRouter, accessSvc); err != nil {
		return err
	}

	if err := e.registerScanHandlers(abuseRouter, accessSvc); err != nil {
		return err
	}

	if err := e.registerSearchHandlers(abuseRouter, accessSvc); err != nil {
		return err
	}

	if err := e.registerAnalyticsHandlers(abuseRouter, accessSvc); err != nil {
		return err
	}

	if err := e.registerBlockListHandlers(abuseRouter, accessSvc); err != nil {
		return err
	}

	return nil
}
