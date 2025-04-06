package models

import (
	"go.lumeweb.com/portal-plugin-abuse/internal/util"
	"time"
)

type CommunicationHourlyCount struct {
	HourlyInterval string `gorm:"column:hourly_interval"`
	CommCount      int64  `gorm:"column:comm_count"`
}

func (CommunicationHourlyCount) TableName() string {
	return "abuse_communication_hourly_counts"
}

// GetTime parses the hourly interval into time.Time using util.ParseTime
func (c *CommunicationHourlyCount) GetTime() (time.Time, error) {
	return util.ParseTime(c.HourlyInterval)
}

// SetTime formats a time.Time for storage
func (c *CommunicationHourlyCount) SetTime(t time.Time) {
	c.HourlyInterval = t.Format("2006-01-02 15:04:05")
}
