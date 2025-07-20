package models

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"strings"
	"time"
)

// ClassificationScores stores detailed scoring information from the classification process
type ClassificationScores struct {
	// Individual category scores
	SpamScore       int `json:"spam_score"`
	HarassmentScore int `json:"harassment_score"`
	ContentScore    int `json:"content_score"`
	OtherScore      int `json:"other_score"`

	// Confidence level (0.0-1.0)
	Confidence float64 `json:"confidence"`

	// Mixed category flag
	IsMixedCategory bool `json:"is_mixed_category"`

	// Secondary categories in order of relevance
	SecondaryCategories []string `json:"secondary_categories,omitempty"`

	// Key terms that triggered classification
	KeyTerms []string `json:"key_terms,omitempty"`

	// Reason for classification (human-readable summary)
	ScoreReason string `json:"score_reason,omitempty"`
}

// RiskFactors stores security and risk indicators from analysis
type RiskFactors struct {
	// Header analysis
	HeaderIssues []string `json:"header_issues,omitempty"`
	HeaderScore  int      `json:"header_score"`

	// Attachment risks
	HasRiskyAttachments   bool     `json:"has_risky_attachments"`
	RiskyAttachmentCount  int      `json:"risky_attachment_count"`
	RiskyAttachmentTypes  []string `json:"risky_attachment_types,omitempty"`
	AttachmentRiskScore   int      `json:"attachment_risk_score"`
	SuspiciousAttachments []string `json:"suspicious_attachments,omitempty"`

	// URL analysis
	HasExternalURLs bool     `json:"has_external_urls"`
	URLCount        int      `json:"url_count"`
	SuspiciousURLs  []string `json:"suspicious_urls,omitempty"`
	URLRiskScore    int      `json:"url_risk_score"`
}

// Case is the core model for abuse reports
type Case struct {
	gorm.Model
	ReferenceNumber string
	Type            CaseType
	Status          CaseStatus
	Priority        CasePriority
	Description     string
	Source          ReportSource

	// Report quality fields
	IsDuplicate bool

	// Review flag
	NeedsReview bool
	// Classification metadata - stored as JSON
	ClassificationScores datatypes.JSON `gorm:"type:json"`
	RiskFactors          datatypes.JSON `gorm:"type:json"`

	// Core References
	ReporterID uint     `gorm:"index,column:reporter_id"`
	Reporter   Reporter `gorm:"foreignKey:ReporterID"`

	SubjectID uint    `gorm:"index,column:subject_id"`
	Subject   Subject `gorm:"foreignKey:SubjectID"`

	AssigneeID *uint

	// Metadata fields
	LastActivityAt time.Time

	// Related collections
	Communications []Communication     `gorm:"foreignKey:CaseID"`
	CaseScans      []CaseScan          `gorm:"foreignKey:CaseID"`
	StatusHistory  []CaseStatusHistory `gorm:"foreignKey:CaseID"`
	Evidence       []Evidence          `gorm:"foreignKey:CaseID"`
}

// Validate validates a Case model
func (c *Case) Validate() error {
	// Validate Type
	if string(c.Type) == "" {
		return fmt.Errorf("case type is required")
	}

	validTypes := make(map[CaseType]bool)
	for _, t := range ValidCaseTypes {
		validTypes[t] = true
	}

	if !validTypes[c.Type] {
		return fmt.Errorf("invalid case type: %s", c.Type)
	}

	// Validate Status
	if string(c.Status) == "" {
		return fmt.Errorf("case status is required")
	}

	if !ValidCaseStatusMap[c.Status] && c.Status != "" {
		return fmt.Errorf("invalid case status: %s", c.Status)
	}

	// Validate Priority
	if string(c.Priority) == "" {
		return fmt.Errorf("case priority is required")
	}

	if !ValidCasePriorityMap[c.Priority] && c.Priority != "" {
		return fmt.Errorf("invalid case priority: %s", c.Priority)
	}

	// Validate Description (minimal validation for now)
	if strings.TrimSpace(c.Description) == "" {
		return fmt.Errorf("case description is required")
	}

	// Validate Source
	validSources := make(map[ReportSource]bool)
	for _, s := range ValidReportSources {
		validSources[ReportSource(s)] = true
	}

	if !validSources[c.Source] && string(c.Source) != "" {
		return fmt.Errorf("invalid report source: %s", c.Source)
	}

	// Ensure Subject exists
	if c.SubjectID == 0 {
		return fmt.Errorf("subject is required")
	}

	return nil
}

// TableName specifies the database table name
func (Case) TableName() string {
	return "abuse_cases"
}

// BeforeCreate generates a human-readable reference number for cases
func (c *Case) BeforeCreate(tx *gorm.DB) error {
	if err := c.Validate(); err != nil {
		return err
	}

	// Generate raw UUID reference without prefix
	_uuid := strings.Replace(uuid.New().String(), "-", "", -1)
	c.ReferenceNumber = _uuid[:12] // First 12 characters of UUID
	c.LastActivityAt = time.Now()
	return nil
}

func (c *Case) BeforeUpdate(tx *gorm.DB) error {
	if err := c.Validate(); err != nil {
		return err
	}

	c.LastActivityAt = time.Now()
	return nil
}

// GetClassificationScores returns the parsed ClassificationScores object
func (c *Case) GetClassificationScores() (*ClassificationScores, error) {
	if c.ClassificationScores == nil || len(c.ClassificationScores) == 0 {
		return &ClassificationScores{}, nil
	}

	var scores ClassificationScores
	err := json.Unmarshal(c.ClassificationScores, &scores)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal classification scores: %w", err)
	}
	return &scores, nil
}

// SetClassificationScores sets the ClassificationScores field from a ClassificationScores object
func (c *Case) SetClassificationScores(scores *ClassificationScores) error {
	if scores == nil {
		c.ClassificationScores = nil
		return nil
	}

	data, err := json.Marshal(scores)
	if err != nil {
		return fmt.Errorf("failed to marshal classification scores: %w", err)
	}

	c.ClassificationScores = datatypes.JSON(data)
	return nil
}

// GetRiskFactors returns the parsed RiskFactors object
func (c *Case) GetRiskFactors() (*RiskFactors, error) {
	if c.RiskFactors == nil || len(c.RiskFactors) == 0 {
		return &RiskFactors{}, nil
	}

	var factors RiskFactors
	err := json.Unmarshal(c.RiskFactors, &factors)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal risk factors: %w", err)
	}
	return &factors, nil
}

// SetRiskFactors sets the RiskFactors field from a RiskFactors object
func (c *Case) SetRiskFactors(factors *RiskFactors) error {
	if factors == nil {
		c.RiskFactors = nil
		return nil
	}

	data, err := json.Marshal(factors)
	if err != nil {
		return fmt.Errorf("failed to marshal risk factors: %w", err)
	}

	c.RiskFactors = datatypes.JSON(data)
	return nil
}
