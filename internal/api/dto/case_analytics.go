package dto

import (
	"fmt"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"strings"
	"time"

	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
)

var _ httputil.DTOResponse[*typesSvc.StatusFlowGraph] = (*StatusFlowGraphResponse)(nil)

type Distribution struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

func (d *Distribution) FromModel(name string, count int64) {
	d.Name = strings.ToLower(name)
	d.Count = count
}

func (d *Distribution) FromUintID(id uint, count int64) {
	d.Name = fmt.Sprintf("%d", id)
	d.Count = count
}

type ResolutionTrend struct {
	Date           time.Time `json:"date"`
	ResolvedCount  int64     `json:"resolved_count"`
	AverageSeconds int64     `json:"average_seconds"`
}

type ResolutionMetrics struct {
	AverageSeconds int64             `json:"average_seconds"`
	DailyTrends    []ResolutionTrend `json:"daily_trends"`
}

type CommunicationsMetrics struct {
	AverageResponseSeconds int64          `json:"average_response_seconds"`
	MaxResponseSeconds     int64          `json:"max_response_seconds"`
	CountsPerCase          []Distribution `json:"counts_per_case"`
}

type EvidenceMetrics struct {
	FilesPerCase   []Distribution `json:"files_per_case"`
	AveragePerCase float64        `json:"average_per_case"`
}

type BlocklistMetrics struct {
	TotalBlocks int64          `json:"total_blocks"`
	ByReason    []Distribution `json:"by_reason"`
	BySeverity  []Distribution `json:"by_severity"`
}

type DurationDistribution struct {
	Status   string  `json:"status"`
	Duration float64 `json:"duration_seconds"`
}

type StatusFlowGraphResponse struct {
	Nodes []StatusFlowNode `json:"nodes"`
	Links []StatusFlowLink `json:"links"`
}

var _ httputil.DTOResponse[[]models.CaseTypeSourceBreakdown] = (*CaseTypeSourceMatrixResponse)(nil)

type CaseTypeSourceMatrixItemResponse struct {
	CaseDate     string `json:"case_date"`
	CaseType     string `json:"case_type"`
	ReportSource string `json:"report_source"`
	CaseCount    int64  `json:"case_count"`
}

type CaseTypeSourceMatrixResponse struct {
	Items []CaseTypeSourceMatrixItemResponse `json:"items"`
}

func (r *CaseTypeSourceMatrixResponse) FromModel(models []models.CaseTypeSourceBreakdown) error {
	r.Items = make([]CaseTypeSourceMatrixItemResponse, len(models))
	for i, model := range models {
		r.Items[i] = CaseTypeSourceMatrixItemResponse{
			CaseDate:     model.CaseDate,
			CaseType:     string(model.CaseType),
			ReportSource: string(model.ReportSource),
			CaseCount:    model.CaseCount,
		}
	}
	return nil
}

type StatusFlowNode struct {
	Name string `json:"name"`
}

type StatusFlowLink struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Value  int64  `json:"value"`
}

// FromModel converts a StatusFlowGraph model to a response DTO
func (r *StatusFlowGraphResponse) FromModel(model *typesSvc.StatusFlowGraph) error {
	r.Nodes = make([]StatusFlowNode, len(model.Nodes))
	for i, node := range model.Nodes {
		r.Nodes[i] = StatusFlowNode{
			Name: node.Name,
		}
	}

	r.Links = make([]StatusFlowLink, len(model.Links))
	for i, link := range model.Links {
		r.Links[i] = StatusFlowLink{
			Source: link.Source,
			Target: link.Target,
			Value:  link.Value,
		}
	}
	return nil
}

type CaseAnalyticsResponse struct {
	TotalCases            int64                  `json:"total_cases"`
	OpenCases             int64                  `json:"open_cases"`
	NewCases              int64                  `json:"new_cases"`
	NeedsReviewCount      int64                  `json:"needs_review_count"`
	StatusDistribution    []Distribution         `json:"status_distribution"`
	CaseTypeDistribution  []Distribution         `json:"case_type_distribution"`
	SourceDistribution    []Distribution         `json:"source_distribution"`
	ResolutionMetrics     ResolutionMetrics      `json:"resolution_metrics"`
	StatusDurations       []DurationDistribution `json:"status_durations"`
	AvgStatusDurations    []DurationDistribution `json:"avg_status_durations"`
	CommunicationsMetrics CommunicationsMetrics  `json:"communications"`
	EvidenceMetrics       EvidenceMetrics        `json:"evidence"`
	BlocklistMetrics      BlocklistMetrics       `json:"blocklist"`
}

func (r *CaseAnalyticsResponse) FromModel(analytics *typesSvc.CaseAnalytics) error {
	r.TotalCases = analytics.TotalCases
	r.OpenCases = analytics.OpenCases
	r.NewCases = analytics.NewCasesInRange
	r.NeedsReviewCount = analytics.NeedsReviewCount
	r.ResolutionMetrics.AverageSeconds = int64(analytics.AvgResolutionSeconds)

	r.StatusDistribution = convertDistribution(analytics.StatusBreakdown)
	r.CaseTypeDistribution = convertDistribution(analytics.CaseTypeBreakdown)
	r.SourceDistribution = convertDistribution(analytics.SourceBreakdown)

	// Convert resolution trends
	if analytics.ResolutionTrends != nil {
		for date, count := range analytics.ResolutionTrends {
			r.ResolutionMetrics.DailyTrends = append(r.ResolutionMetrics.DailyTrends, ResolutionTrend{
				Date:          date,
				ResolvedCount: count,
			})
		}
	}

	// Status durations
	for status, duration := range analytics.StatusDurations {
		r.StatusDurations = append(r.StatusDurations, DurationDistribution{
			Status:   strings.ToLower(string(status)),
			Duration: duration.Seconds(),
		})
	}

	// Average status durations
	for status, avgDuration := range analytics.AvgStatusDurations {
		r.AvgStatusDurations = append(r.AvgStatusDurations, DurationDistribution{
			Status:   strings.ToLower(string(status)),
			Duration: avgDuration.Seconds(),
		})
	}

	// Communications metrics
	if analytics.CommsMetrics.CommsPerCase != nil {
		r.CommunicationsMetrics.CountsPerCase = convertUintDistribution(analytics.CommsMetrics.CommsPerCase)
	} else {
		r.CommunicationsMetrics.CountsPerCase = []Distribution{}
	}
	r.CommunicationsMetrics.AverageResponseSeconds = int64(analytics.CommsMetrics.AvgResponseTime.Seconds())
	r.CommunicationsMetrics.MaxResponseSeconds = int64(analytics.CommsMetrics.MaxResponseTime.Seconds())

	// Evidence metrics
	if analytics.EvidenceMetrics.FilesPerCase != nil {
		r.EvidenceMetrics.FilesPerCase = convertUintDistribution(analytics.EvidenceMetrics.FilesPerCase)
	}
	r.EvidenceMetrics.AveragePerCase = analytics.EvidenceMetrics.AvgFilesPerCase

	// Blocklist metrics
	r.BlocklistMetrics.TotalBlocks = analytics.BlocklistMetrics.TotalBlocks
	r.BlocklistMetrics.ByReason = convertDistribution(analytics.BlocklistMetrics.BlocksByReason)
	r.BlocklistMetrics.BySeverity = convertDistribution(analytics.BlocklistMetrics.BlocksBySeverity)

	return nil
}

func convertDistribution[T ~string](input map[T]int64) []Distribution {
	var result []Distribution
	for k, v := range input {
		result = append(result, Distribution{
			Name:  strings.ToLower(string(k)),
			Count: v,
		})
	}
	return result
}

func convertUintDistribution(input map[uint]int64) []Distribution {
	var result []Distribution
	for k, v := range input {
		result = append(result, Distribution{
			Name:  fmt.Sprintf("%d", k),
			Count: v,
		})
	}
	return result
}
