package dto

import (
	z "github.com/Oudwins/zog"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-plugin-abuse/internal/config"
)

var _ httputil.DTOValidator = (*ProviderTemplateCreateRequest)(nil)
var _ httputil.DTOValidator = (*ProviderTemplateUpdateRequest)(nil)
var _ httputil.DTORequest[*config.ProviderTemplateConfig] = (*ProviderTemplateCreateRequest)(nil)
var _ httputil.DTORequest[*config.ProviderTemplateConfig] = (*ProviderTemplateUpdateRequest)(nil)
var _ httputil.DTOResponse[*config.ProviderTemplateConfig] = (*ProviderTemplateResponse)(nil)

// ProviderTemplateCreateRequest holds data for creating a provider email template
type ProviderTemplateCreateRequest struct {
	Name            string   `json:"name"`
	Enabled         bool     `json:"enabled" default:"true"`
	Priority        int      `json:"priority" default:"100"`
	DomainPatterns  []string `json:"domain_patterns"`
	HeaderPatterns  []string `json:"header_patterns"`
	ContentPatterns []string `json:"content_patterns"`
}

// ProviderTemplateUpdateRequest holds data for updating a provider email template
type ProviderTemplateUpdateRequest struct {
	Name            *string   `json:"name,omitempty"`
	Enabled         *bool     `json:"enabled,omitempty"`
	Priority        *int      `json:"priority,omitempty"`
	DomainPatterns  *[]string `json:"domain_patterns,omitempty"`
	HeaderPatterns  *[]string `json:"header_patterns,omitempty"`
	ContentPatterns *[]string `json:"content_patterns,omitempty"`
}

func (r *ProviderTemplateCreateRequest) Schema() *z.StructSchema {
	return z.Struct(z.Schema{
		"Name":             z.String().Required().Min(2),
		"Enabled":          z.Bool().Optional().Default(true),
		"Priority":         z.Int().Optional().Default(100).GT(0),
		"DomainPatterns":  z.Slice(z.String().Required().Min(1)),
		"HeaderPatterns":  z.Slice(z.String()),
		"ContentPatterns": z.Slice(z.String()),
	})
}

func (r *ProviderTemplateUpdateRequest) Schema() *z.StructSchema {
	return z.Struct(z.Schema{
		"Name":            z.Ptr(z.String().Optional().Min(2)),
		"Enabled":         z.Ptr(z.Bool().Optional()),
		"Priority":        z.Ptr(z.Int().Optional().GT(0)),
		"DomainPatterns": z.Ptr(z.Slice(z.String().Required().Min(1))),
		"HeaderPatterns": z.Ptr(z.Slice(z.String())),
		"ContentPatterns": z.Ptr(z.Slice(z.String())),
	})
}

// ProviderTemplateResponse holds response data for provider template operations
type ProviderTemplateResponse struct {
	Name            string   `json:"name"`
	Enabled         bool     `json:"enabled"`
	Priority        int      `json:"priority"`
	DomainPatterns  []string `json:"domain_patterns"`
	HeaderPatterns  []string `json:"header_patterns"`
	ContentPatterns []string `json:"content_patterns"`
}

func (r *ProviderTemplateCreateRequest) ToModel() (*config.ProviderTemplateConfig, error) {
	return &config.ProviderTemplateConfig{
		Name:            r.Name,
		Enabled:         r.Enabled,
		Priority:        r.Priority,
		DomainPatterns:  r.DomainPatterns,
		HeaderPatterns:  r.HeaderPatterns,
		ContentPatterns: r.ContentPatterns,
	}, nil
}

func (r *ProviderTemplateUpdateRequest) ToModel() (*config.ProviderTemplateConfig, error) {
	cfg := &config.ProviderTemplateConfig{}
	
	if r.Name != nil {
		cfg.Name = *r.Name
	}
	if r.Enabled != nil {
		cfg.Enabled = *r.Enabled
	}
	if r.Priority != nil {
		cfg.Priority = *r.Priority
	}
	if r.DomainPatterns != nil {
		cfg.DomainPatterns = *r.DomainPatterns
	}
	if r.HeaderPatterns != nil {
		cfg.HeaderPatterns = *r.HeaderPatterns
	}
	if r.ContentPatterns != nil {
		cfg.ContentPatterns = *r.ContentPatterns
	}
	
	return cfg, nil
}

func (r *ProviderTemplateResponse) FromModel(cfg *config.ProviderTemplateConfig) error {
	r.Name = cfg.Name
	r.Enabled = cfg.Enabled
	r.Priority = cfg.Priority
	r.DomainPatterns = cfg.DomainPatterns
	r.HeaderPatterns = cfg.HeaderPatterns
	r.ContentPatterns = cfg.ContentPatterns
	return nil
}
