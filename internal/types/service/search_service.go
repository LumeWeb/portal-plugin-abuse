package service

import (
	"context"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/queryutil"
)

// SearchService defines the interface for search operations
type GlobalSearchResult struct {
	Cases struct {
		Items []*models.Case
		Total int64
	}
	Reporters struct {
		Items []*models.Reporter
		Total int64
	}
	Pagination queryutil.Pagination
}

type SearchService interface {
	core.Service

	// SearchCases performs a search across cases
	SearchCases(ctx context.Context, query string, filters []queryutil.CrudFilter, pagination queryutil.Pagination) ([]models.Case, int64, error)

	// SearchReporters performs a search across reporters
	SearchReporters(ctx context.Context, query string, filters []queryutil.CrudFilter, pagination queryutil.Pagination) ([]models.Reporter, int64, error)

	// GlobalSearch performs a search across different entities
	GlobalSearch(ctx context.Context, query string, pagination queryutil.Pagination) (*GlobalSearchResult, error)
}
