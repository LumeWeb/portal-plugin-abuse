package models

import (
	"fmt"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Evidence represents a file or data submitted as evidence for a case
type Evidence struct {
	gorm.Model
	CaseID      uint
	Case        Case
	SubmitterID uint // Reporter or user who submitted this evidence

	// File information
	FileName    string
	ContentType string
	FileSize    int64
	StoragePath string // Full S3 path to the evidence file

	// Metadata
	Source      EvidenceSource
	Description string // Optional description
	Metadata    datatypes.JSON
}

// EvidenceSource defines where the evidence came from
type EvidenceSource string

const (
	EvidenceSourceEmail     EvidenceSource = "email"
	EvidenceSourceWebUpload EvidenceSource = "web_upload"
	EvidenceSourceAPI       EvidenceSource = "api"
	EvidenceSourceSystem    EvidenceSource = "system"
)

// TableName specifies the database table name
func (Evidence) TableName() string {
	return "abuse_evidence"
}

// Validate performs validation on the evidence model
func (e *Evidence) Validate() error {
	// Case ID is required
	if e.CaseID == 0 {
		return fmt.Errorf("case ID is required")
	}

	// Validate evidence source
	validSources := map[EvidenceSource]bool{
		EvidenceSourceEmail:     true,
		EvidenceSourceWebUpload: true,
		EvidenceSourceAPI:       true,
		EvidenceSourceSystem:    true,
	}

	if !validSources[e.Source] {
		return fmt.Errorf("invalid evidence source: %s", e.Source)
	}

	return nil
}

// BeforeCreate validates the evidence before creation
func (e *Evidence) BeforeCreate(tx *gorm.DB) error {
	return e.Validate()
}

// BeforeUpdate validates the evidence before update
func (e *Evidence) BeforeUpdate(tx *gorm.DB) error {
	return e.Validate()
}
