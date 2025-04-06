package models

import (
	"fmt"
	"go.lumeweb.com/portal-plugin-abuse/internal/util"
	"time"
)

type CaseStatusBreakdown struct {
	Status    CaseStatus `gorm:"column:status"`
	CreatedAt time.Time  `gorm:"column:created_at"`
	Count     int64      `gorm:"column:count"`
}

func (CaseStatusBreakdown) TableName() string {
	return "abuse_case_status_breakdown"
}

type CaseTypeBreakdown struct {
	Type      CaseType  `gorm:"column:type"`
	CreatedAt time.Time `gorm:"column:created_at"`
	Count     int64     `gorm:"column:count"`
}

func (CaseTypeBreakdown) TableName() string {
	return "abuse_case_type_breakdown"
}

type CaseSourceBreakdown struct {
	Source    ReportSource `gorm:"column:source"`
	CreatedAt time.Time    `gorm:"column:created_at"`
	Count     int64        `gorm:"column:count"`
}

func (CaseSourceBreakdown) TableName() string {
	return "abuse_case_source_breakdown"
}

type DurationDistribution struct {
	Status    CaseStatus    `gorm:"column:status"`
	Duration  time.Duration `gorm:"column:duration"`
	CaseCount int64         `gorm:"column:case_count"`
}

func (DurationDistribution) TableName() string {
	return "abuse_case_duration_distribution"
}

type CaseStatusTransition struct {
	ChangedAt       time.Time  `gorm:"column:changed_at"`
	CaseType        CaseType   `gorm:"column:case_type"`
	TransitionDate  string     `gorm:"column:transition_date"`
	FromStatus      CaseStatus `gorm:"column:from_status"` // old_status in view
	ToStatus        CaseStatus `gorm:"column:to_status"`   // new_status in view
	TransitionCount int64      `gorm:"column:transition_count"`
	CaseID          uint       `gorm:"column:case_id"` // Add case_id from view
}

func (CaseStatusTransition) TableName() string {
	return "abuse_case_status_transitions"
}

type CaseTypeSourceBreakdown struct {
	CaseDate     string       `gorm:"column:case_date"`
	CaseType     CaseType     `gorm:"column:case_type"`
	ReportSource ReportSource `gorm:"column:report_source"`
	Priority     CasePriority `gorm:"column:priority"`
	CaseCount    int64        `gorm:"column:case_count"`
}

func (CaseTypeSourceBreakdown) TableName() string {
	return "abuse_case_type_source_breakdown"
}

// CaseDailyMetrics represents the aggregated daily case metrics from the database view
type CaseDailyMetrics struct {
	MetricDate    string       `gorm:"column:metric_date"` // Stored as string for DB compatibility
	OpenCases     int64        `gorm:"column:open_cases"`
	NewCases      int64        `gorm:"column:new_cases"`
	ResolvedCases int64        `gorm:"column:resolved_cases"`
	CaseType      CaseType     `gorm:"column:case_type"`
	Priority      CasePriority `gorm:"column:priority"`
	Source        ReportSource `gorm:"column:source"`
}

// GetDate parses the metric date into time.Time using util.ParseTime
func (d *CaseDailyMetrics) GetDate() (time.Time, error) {
	return util.ParseTime(d.MetricDate)
}

// TableName specifies the database view name
func (CaseDailyMetrics) TableName() string {
	return "abuse_case_daily_metrics"
}

// CaseDailyResolution represents the daily case resolution metrics from the database view
type CaseDailyResolution struct {
	ResolutionDate       string  `gorm:"column:resolution_date"` // Stored as string for DB compatibility
	ResolvedCount        int64   `gorm:"column:resolved_count"`
	AvgResolutionSeconds float64 `gorm:"column:avg_resolution_seconds"`
}

// GetDate parses the resolution date into time.Time
func (d *CaseDailyResolution) GetDate() (time.Time, error) {
	// Try parsing as RFC3339 first
	t, err := time.Parse(time.RFC3339, d.ResolutionDate)
	if err == nil {
		return t, nil
	}

	// Try parsing as simple date
	t, err = time.Parse("2006-01-02", d.ResolutionDate)
	if err == nil {
		return t, nil
	}

	// Try parsing as MySQL DATETIME format
	t, err = time.Parse("2006-01-02 15:04:05", d.ResolutionDate)
	if err == nil {
		return t, nil
	}

	// Try parsing with nanoseconds if present
	t, err = time.Parse("2006-01-02 15:04:05.999999999", d.ResolutionDate)
	if err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("failed to parse date '%s': %v", d.ResolutionDate, err)
}

// SetDate formats a time.Time for storage
func (d *CaseDailyResolution) SetDate(t time.Time) {
	d.ResolutionDate = t.Format("2006-01-02") // Standardize on YYYY-MM-DD format
}

// TableName specifies the database view name
func (CaseDailyResolution) TableName() string {
	return "abuse_case_daily_resolutions"
}

// CaseDurationDistribution represents case duration metrics from the view
type CaseDurationDistribution struct {
	Status    CaseStatus `gorm:"column:status"`
	Duration  float64    `gorm:"column:duration"` // Duration in seconds
	CaseCount int64      `gorm:"column:case_count"`
}

// TableName specifies the database view name
func (CaseDurationDistribution) TableName() string {
	return "abuse_case_duration_distribution"
}

// DurationAsDuration converts seconds to time.Duration
func (c *CaseDurationDistribution) DurationAsDuration() time.Duration {
	return time.Duration(c.Duration * float64(time.Second))
}
