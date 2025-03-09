package email

import (
	"sync"

	"github.com/mnako/letters"
	"go.lumeweb.com/portal/core"
	"go.uber.org/zap"
)

// ProviderTemplateRegistry manages email provider templates
type ProviderTemplateRegistry struct {
	ctx        core.Context
	logger     *core.Logger
	mutex      sync.RWMutex
	templates  map[string]ProviderTemplate
	detectors  map[string]ProviderDetector
	priorities map[string]int // Lower number = higher priority
}

// ProviderDetector is a function that detects if an email matches a provider
type ProviderDetector func(email *letters.Email) bool

// NewProviderTemplateRegistry creates a new provider template registry
func NewProviderTemplateRegistry(ctx core.Context) *ProviderTemplateRegistry {
	return &ProviderTemplateRegistry{
		ctx:        ctx,
		logger:     ctx.NamedLogger("provider-registry"),
		templates:  make(map[string]ProviderTemplate),
		detectors:  make(map[string]ProviderDetector),
		priorities: make(map[string]int),
	}
}

// RegisterProvider registers a provider template with the registry
func (r *ProviderTemplateRegistry) RegisterProvider(
	providerID string,
	template ProviderTemplate,
	detector ProviderDetector,
	priority int,
) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.templates[providerID] = template
	r.detectors[providerID] = detector
	r.priorities[providerID] = priority

	r.logger.Info("Registered provider template",
		zap.String("provider", providerID),
		zap.Int("priority", priority))
}

// UnregisterProvider removes a provider from the registry
func (r *ProviderTemplateRegistry) UnregisterProvider(providerID string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	delete(r.templates, providerID)
	delete(r.detectors, providerID)
	delete(r.priorities, providerID)

	r.logger.Info("Unregistered provider template",
		zap.String("provider", providerID))
}

// DetectProvider tries to detect which provider an email is from
func (r *ProviderTemplateRegistry) DetectProvider(email *letters.Email) (string, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// First, get a slice of all providers sorted by priority
	type providerPriority struct {
		ID       string
		Priority int
	}

	providers := make([]providerPriority, 0, len(r.templates))
	for id, _ := range r.templates {
		providers = append(providers, providerPriority{
			ID:       id,
			Priority: r.priorities[id],
		})
	}

	// Sort by priority (lower number = higher priority)
	for i := 0; i < len(providers)-1; i++ {
		for j := i + 1; j < len(providers); j++ {
			if providers[j].Priority < providers[i].Priority {
				providers[i], providers[j] = providers[j], providers[i]
			}
		}
	}

	// Try each provider in priority order
	for _, provider := range providers {
		detector, ok := r.detectors[provider.ID]
		if !ok {
			continue
		}

		if detector(email) {
			return provider.ID, true
		}
	}

	return "", false
}

// GetTemplate retrieves a provider template by ID
func (r *ProviderTemplateRegistry) GetTemplate(providerID string) (ProviderTemplate, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	template, ok := r.templates[providerID]
	return template, ok
}

// GetAllProviders returns a list of all registered providers
func (r *ProviderTemplateRegistry) GetAllProviders() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	providers := make([]string, 0, len(r.templates))
	for id := range r.templates {
		providers = append(providers, id)
	}

	return providers
}
