// Package plugin provides the main entry point for the abuse plugin
package plugin

import (
	"embed"
	"go.lumeweb.com/portal-plugin-abuse/build"
	"go.lumeweb.com/portal-plugin-abuse/internal"
	"go.lumeweb.com/portal-plugin-abuse/internal/api"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/migrations"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal-plugin-abuse/internal/service"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal-plugin-abuse/internal/workflow"
	"go.lumeweb.com/portal/core"
	coreService "go.lumeweb.com/portal/service"
	"io/fs"
	"os"
)

//go:embed templates/*.tpl
var emailTemplates embed.FS

// init registers the plugin with the Portal framework
// This is called automatically when the plugin is loaded
func init() {
	templates, err := coreService.MailerTemplatesFromEmbed(&emailTemplates, "templates")
	if err != nil {
		panic(err)
	}

	var _fs fs.FS
	var _reportFs fs.FS

	envPath := os.Getenv("ABUSE_PLUGIN_BUNDLE_PATH")
	reportEnvPath := os.Getenv("ABUSE_REPORT_PLUGIN_BUNDLE_PATH")

	if envPath != "" {
		_fs = core.NewWebBundleLiveFS(envPath)
	}

	if reportEnvPath != "" {
		_reportFs = core.NewWebBundleLiveFS(reportEnvPath)
	}

	if _fs == nil {
		panic("plugin: no plugin bundle found")
	}

	if _reportFs == nil {
		panic("plugin: no plugin bundle found")
	}

	core.RegisterPlugin(core.PluginInfo{
		ID:      internal.PLUGIN_NAME,
		Version: build.GetInfo(),
		Meta: func(ctx core.Context, builder core.PortalMetaBuilder) error {
			builder.AddFeatureFlag(internal.PLUGIN_NAME, true)
			return nil
		},
		API: api.NewAbuseAPI,
		APIExtensions: func(ctx core.Context) ([]core.APIExtensionFactory, error) {
			return []core.APIExtensionFactory{
				api.NewAdminExtension(),
			}, nil
		},
		Models: []any{
			&models.Case{},
			&models.Reporter{},
			&models.Subject{},
			&models.CaseScan{},
			&models.Communication{},
			&models.CaseScan{},
			&models.Evidence{},
			&models.BlockList{},
		},
		Migrations: core.DBMigration{
			core.DB_TYPE_MYSQL:  migrations.GetMySQL(),
			core.DB_TYPE_SQLITE: migrations.GetSQLite(),
		},
		MailerTemplates: templates,
		Services: func() ([]core.ServiceInfo, error) {
			return []core.ServiceInfo{
				{
					ID:      typesSvc.REPORTER_SERVICE,
					Factory: service.NewReporterService,
				},
				{
					ID:      typesSvc.SUBJECT_SERVICE,
					Factory: service.NewSubjectService,
				},
				{
					ID:      typesSvc.TOKEN_SERVICE,
					Factory: service.NewTokenService,
					Depends: []string{typesSvc.REPORTER_SERVICE},
				},
				{
					ID:      typesSvc.CASE_SERVICE,
					Factory: service.NewCaseService,
					Depends: []string{
						core.HTTP_SERVICE,
					},
				},
				{
					ID:      typesSvc.COMMUNICATION_SERVICE,
					Factory: service.NewCommunicationService,
					Depends: []string{
						typesSvc.CASE_SERVICE,
						typesSvc.EMAIL_SERVICE,
					},
				},
				{
					ID:      typesSvc.EMAIL_SERVICE,
					Factory: service.NewEmailService,
					Depends: []string{
						core.MAILER_SERVICE,
					},
				},
				{
					ID:      typesSvc.ABUSE_REPORT_SERVICE,
					Factory: service.NewAbuseReportService,
					Depends: []string{
						typesSvc.CASE_SERVICE,
						typesSvc.REPORTER_SERVICE,
						typesSvc.SUBJECT_SERVICE,
					},
				},
				{
					ID:      typesSvc.BLOCKLIST_SERVICE,
					Factory: service.NewBlockListService,
					Depends: []string{typesSvc.CASE_SERVICE},
				},
				{
					ID:      typesSvc.SCAN_SERVICE,
					Factory: service.NewScanService,
					Depends: []string{
						typesSvc.CASE_SERVICE,
						core.CONTENT_SCANNER_SERVICE,
						typesSvc.SUBJECT_SERVICE,
						typesSvc.REPORTER_SERVICE,
						typesSvc.EMAIL_SERVICE,
						core.WORKFLOW_SERVICE,
						core.CRON_SERVICE,
						core.STORAGE_SERVICE,
						core.UPLOAD_SERVICE,
					},
				},
				{
					ID:      typesSvc.EVIDENCE_SERVICE,
					Factory: service.NewEvidenceService,
					Depends: []string{
						core.STORAGE_SERVICE,
						typesSvc.CASE_SERVICE,
						typesSvc.REPORTER_SERVICE,
						typesSvc.EMAIL_SERVICE,
					},
				},
				{
					ID:      typesSvc.SEARCH_SERVICE,
					Factory: service.NewSearchService,
				},
			}, nil
		},
		WebBundles: core.NewWebBundles(core.NewWebBundle(_fs, core.WithWebBundleTargetApps("admin")), core.NewWebBundle(_reportFs, core.WithWebBundleTargetApps("abuse"))),
		Operations: func(ctx core.Context) ([]core.Operation, error) {
			return []core.Operation{
				workflow.NewAbuseScanOperation(ctx),
			}, nil
		},
		Workflows: func(ctx core.Context) ([]core.WorkflowDefinition, error) {
			return []core.WorkflowDefinition{
				{
					Name: workflow.AbuseScanWorkflowName,
					Steps: []core.OperationStep{
						{
							Operation:       workflow.AbuseScanOperationName,
							FailureBehavior: core.RetryStep,
							Foreground:      false,
						},
					},
					AutoTriggerFirstStep: true,
				},
			}, nil
		},
	})
}
