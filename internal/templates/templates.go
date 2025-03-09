package templates

import (
	"embed"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/portal/service"
)

//go:embed templates/*
var mailerTemplates embed.FS

const (
	MAILER_TPL_ITEM_CREATED = "item_created"
)

func GetMailerTemplates() (map[string]core.MailerTemplate, error) {
	return service.MailerTemplatesFromEmbed(&mailerTemplates, "")
}
