package service

import (
	"fmt"
	"go.lumeweb.com/portal/core"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// BaseService provides common functionality for services
type BaseService struct {
	ctx    core.Context
	db     *gorm.DB
	logger *core.Logger
}

// ValidateTemplateData ensures required fields exist in template data
func (b *BaseService) ValidateTemplateData(templateName string, data map[string]interface{}, requiredFields []string) error {
	missing := make([]string, 0)

	for _, field := range requiredFields {
		if _, exists := data[field]; !exists {
			missing = append(missing, field)
		}
	}

	if len(missing) > 0 {
		b.logger.Error("Missing required template fields",
			zap.String("template", templateName),
			zap.Strings("missing", missing))
		if len(missing) == 1 {
			return fmt.Errorf("missing required field: %s", missing[0])
		}
		return fmt.Errorf("missing required fields: %v", missing)
	}

	return nil
}

// InitializeBaseService initializes the BaseService struct
func (b *BaseService) InitializeBaseService(ctx core.Context, service core.Service) {
	b.ctx = ctx
	b.db = ctx.DB()
	b.logger = ctx.ServiceLogger(service)
}
