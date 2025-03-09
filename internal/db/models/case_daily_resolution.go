package models

import (
	"time"
)

// DailyResolution represents the daily case resolution metrics from the database view
type DailyResolution struct {
	ResolutionDate     time.Time `gorm:"column:resolution_date"`
	ResolvedCount      int64     `gorm:"column:resolved_count"`
	AvgResolutionSeconds float64 `gorm:"column:avg_resolution_seconds"`
}

// TableName specifies the database view name
func (DailyResolution) TableName() string {
	return "abuse_daily_resolutions"
}
