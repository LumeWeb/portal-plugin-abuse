package dto

import (
	"fmt"
	"github.com/samber/lo"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
)

var _ httputil.DTOResponse[*typesSvc.GlobalSearchResult] = (*GlobalSearchResponse)(nil)

// ConvertSearchResults handles model-to-DTO conversion for search results
func ConvertSearchResults[T any, D any](items []T, newDTO func() httputil.DTOResponse[T]) []D {
	results := make([]D, len(items))
	for i, item := range items {
		dto := newDTO()
		err := dto.FromModel(item)
		if err != nil {
			panic(fmt.Sprintf("conversion failed: %v", err))
		}

		results[i] = dto.(D) // Type assertion to convert to D
	}
	return results
}

type GlobalSearchResponse struct {
	Cases      []CaseResponse     `json:"cases"`
	Reporters  []ReporterResponse `json:"reporters"`
	TotalCount int64              `json:"total_count"`
}

// FromModel implements DTOResponse interface for the composite search result
func (r *GlobalSearchResponse) FromModel(model *typesSvc.GlobalSearchResult) error {
	r.Cases = lo.FromSlicePtr(ConvertSearchResults[*models.Case, *CaseResponse](
		model.Cases.Items,
		func() httputil.DTOResponse[*models.Case] {
			return &CaseResponse{}
		},
	))

	r.Reporters = lo.FromSlicePtr(ConvertSearchResults[*models.Reporter, *ReporterResponse](
		model.Reporters.Items,
		func() httputil.DTOResponse[*models.Reporter] {
			return &ReporterResponse{}
		},
	))

	r.TotalCount = model.Cases.Total + model.Reporters.Total
	return nil
}
