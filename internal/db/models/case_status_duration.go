package models

import "time"

// CaseStatusDuration represents the calculated duration a case spent in each status
type CaseStatusDuration struct {
	CaseID         uint       `gorm:"column:case_id"`
	NewStatus      CaseStatus `gorm:"column:new_status"`
	DurationSeconds float64    `gorm:"column:duration_seconds"`
}

// Duration converts seconds to time.Duration
func (csd *CaseStatusDuration) Duration() time.Duration {
	return time.Duration(csd.DurationSeconds * float64(time.Second))
}

// TableName specifies the database view name
func (CaseStatusDuration) TableName() string {
	return "abuse_case_status_durations"
}
