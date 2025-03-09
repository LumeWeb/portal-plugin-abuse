package models

import (
	"fmt"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"time"
)

// CaseScan links cases to existing scan results
type CaseScan struct {
	gorm.Model
	CaseID uint
	Case   Case `gorm:"foreignKey:CaseID"`

	// Subject reference contains the content hash/identifier
	SubjectID uint
	Subject   Subject `gorm:"foreignKey:SubjectID"`

	Status       ScanStatus
	Priority     int        // Higher for manual scans
	RequestedBy  uint       // User ID for manual requests
	ScheduledFor time.Time  // When to run scan
	LastAttempt  *time.Time // Last attempt timestamp

	ScanResults datatypes.JSON
}

// Validate validates a CaseScan model
func (c *CaseScan) Validate() error {
	// Validate Subject reference - contains the hash
	if c.SubjectID == 0 {
		return fmt.Errorf("subject reference is required")
	}

	// Validate Status
	validStatuses := map[ScanStatus]bool{
		ScanStatusPending:  true,
		ScanStatusClean:    true,
		ScanStatusFlagged:  true,
		ScanStatusError:    true,
		ScanStatusScanning: true,
	}

	if !validStatuses[c.Status] && string(c.Status) != "" {
		return fmt.Errorf("invalid scan status: %s", c.Status)
	}

	return nil
}

// TableName specifies the database table name
func (CaseScan) TableName() string {
	return "abuse_case_scans"
}

// BeforeCreate sets default values and validates
func (c *CaseScan) BeforeCreate(tx *gorm.DB) error {
	if err := c.Validate(); err != nil {
		return err
	}

	if c.ScheduledFor.IsZero() {
		c.ScheduledFor = time.Now()
	}
	if string(c.Status) == "" {
		c.Status = ScanStatusPending
	}
	return nil
}

// BeforeUpdate validates the scan before update
func (c *CaseScan) BeforeUpdate(tx *gorm.DB) error {
	return c.Validate()
}
