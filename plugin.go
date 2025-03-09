// Package plugin provides the main entry point for the template plugin
package plugin

import (
	"go.lumeweb.com/portal-plugin-template/build"
	"go.lumeweb.com/portal-plugin-template/internal"
	"go.lumeweb.com/portal-plugin-template/internal/api"
	"go.lumeweb.com/portal-plugin-template/internal/protocol"
	"go.lumeweb.com/portal-plugin-template/internal/db/migrations"
	"go.lumeweb.com/portal-plugin-template/internal/db/models"
	"go.lumeweb.com/portal-plugin-template/internal/service"
	"go.lumeweb.com/portal-plugin-template/internal/templates"
	"go.lumeweb.com/portal/core"
)

// init registers the template plugin with the Portal framework
// This is called automatically when the plugin is loaded
func init() {
	tpls, err := templates.GetMailerTemplates()
	if err != nil {
		panic(err)
	}
	core.RegisterPlugin(core.PluginInfo{
		ID:      internal.PLUGIN_NAME,
		Version: build.GetInfo(),
		Meta: func(ctx core.Context, builder core.PortalMetaBuilder) error {
			builder.AddFeatureFlag(internal.PLUGIN_NAME, true)
			return nil
		},
		API: func() (core.API, []core.ContextBuilderOption, error) {
			return api.NewAPI()
		},
		Protocol: func() (core.Protocol, []core.ContextBuilderOption, error) {
			return protocol.NewProtocol()
		},
		Services: func() ([]core.ServiceInfo, error) {
			return []core.ServiceInfo{
				{
					ID:      service.ITEM_SERVICE,
					Factory: service.NewItemService,
				},
			}, nil
		},
		Models: []any{
			&models.Item{},
		},
		Migrations: core.DBMigration{
			core.DB_TYPE_MYSQL:  migrations.GetMySQL(),
			core.DB_TYPE_SQLITE: migrations.GetSQLite(),
		},
		MailerTemplates: tpls,
	})
}
