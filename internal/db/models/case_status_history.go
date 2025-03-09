package models

import (
	"gorm.io/gorm"
	"time"
)

type CaseStatusHistory struct {
	gorm.Model
	CaseID     uint
	OldStatus  CaseStatus
	NewStatus  CaseStatus 
	ChangedAt  time.Time
	ChangedBy  uint // User/System ID (0 = system)
}
