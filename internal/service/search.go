package service

import (
	"context"
	"fmt"
	"github.com/samber/lo"

	"go.lumeweb.com/portal-plugin-abuse/internal/db"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/queryutil"
	"go.uber.org/zap"
)

// SearchServiceDefault implements the SearchService interface
type SearchServiceDefault struct {
	BaseService
}

// Ensure SearchServiceDefault implements the interface
var _ typesSvc.SearchService = (*SearchServiceDefault)(nil)

// NewSearchService creates a new search service
func NewSearchService() (core.Service, []core.ContextBuilderOption, error) {
	svc := &SearchServiceDefault{}

	options := []core.ContextBuilderOption{
		func(ctx core.Context) (core.Context, error) {
			svc.BaseService.InitializeBaseService(ctx, svc)

			return ctx, nil
		},
	}

	return svc, options, nil
}

// ID returns the service identifier
func (s *SearchServiceDefault) ID() string {
	return typesSvc.SEARCH_SERVICE
}

// SearchCases performs a search across cases
func (s *SearchServiceDefault) SearchCases(ctx context.Context, query string, filters []queryutil.Filter, pagination queryutil.Pagination) ([]models.Case, int64, error) {
	var cases []models.Case
	var total int64

	// Apply search query if provided
	if query != "" {
		filters = append(filters, queryutil.Filter{
			Field: "search",
			Value: query,
		})
	}

	if err := db.List(context.Background(), s.ctx, s.db, filters, []queryutil.Sort{}, pagination, &cases, &total); err != nil {
		s.logger.Error("Failed to list cases", zap.Error(err))
		return nil, 0, fmt.Errorf("failed to list cases: %w", err)
	}

	return cases, total, nil
}

// SearchReporters performs a search across reporters
func (s *SearchServiceDefault) SearchReporters(ctx context.Context, query string, filters []queryutil.Filter, pagination queryutil.Pagination) ([]models.Reporter, int64, error) {
	var reporters []models.Reporter
	var total int64

	// Apply search query if provided
	if query != "" {
		filters = append(filters, queryutil.Filter{
			Field: "search",
			Value: query,
		})
	}

	if err := db.List(context.Background(), s.ctx, s.db, filters, []queryutil.Sort{}, pagination, &reporters, &total); err != nil {
		s.logger.Error("Failed to list reporters", zap.Error(err))
		return nil, 0, fmt.Errorf("failed to list reporters: %w", err)
	}

	return reporters, total, nil
}

// GlobalSearch performs a search across different entities
func (s *SearchServiceDefault) GlobalSearch(ctx context.Context, query string, pagination queryutil.Pagination) (*typesSvc.GlobalSearchResult, error) {
	result := &typesSvc.GlobalSearchResult{}

	// Search cases with pagination
	cases, casesTotal, err := s.SearchCases(ctx, query, nil, pagination)
	if err != nil {
		return nil, err
	}
	result.Cases.Items = lo.ToSlicePtr(cases)
	result.Cases.Total = casesTotal

	// Search reporters with same pagination
	reporters, reportersTotal, err := s.SearchReporters(ctx, query, nil, pagination)
	if err != nil {
		return nil, err
	}
	result.Reporters.Items = lo.ToSlicePtr(reporters)
	result.Reporters.Total = reportersTotal

	result.Pagination = pagination

	return result, nil
}
