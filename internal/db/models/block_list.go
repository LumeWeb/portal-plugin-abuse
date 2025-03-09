package models

import (
	mh "github.com/multiformats/go-multihash"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"time"
)

// BlockList represents content that is blocked from being uploaded or shared
type BlockList struct {
	gorm.Model

	// Content identification
	Hash     mh.Multihash // Multihash of content
	MimeType string       // Content type if known
	Size     uint64       // Size if known
	FileName string       // Original filename if known

	// User context
	UploaderID *uint // User who uploaded/shared the content (if known)

	// Block details
	Reason   string // Why it was blocked (malware, policy, etc.)
	Severity string // critical, high, medium, low
	Action   string `gorm:"type:string;not null"`       // reject, quarantine, warn, log

	// Block metadata
	Description string `gorm:"type:text"`         // Human readable explanation
	BlockedBy   uint   `gorm:"not null;index"`    // User ID or 0 for system
	Source      string // Where block originated (scanner, report, manual, etc.)

	// Related abuse case
	CaseID *uint // Associated abuse case if any
	Case   *Case `gorm:"foreignKey:CaseID"`

	// Temporal controls
	ExpiresAt  *time.Time // When block expires (nil = permanent)
	ReviewedAt *time.Time // Last human review timestamp

	// Extended data
	Metadata datatypes.JSON // Additional context (scanner details, etc.)
}

// TableName specifies the database table name
func (BlockList) TableName() string {
	return "abuse_blocked_content"
}

// BlockReason defines why content was blocked
type BlockReason string

const (
	BlockReasonMalware      BlockReason = "malware"     // Security threat
	BlockReasonCsam         BlockReason = "csam"        // Illegal content
	BlockReasonCopyright    BlockReason = "copyright"   // DMCA/copyright violation
	BlockReasonHarassment   BlockReason = "harassment"  // Targeted harassment
	BlockReasonHateSpeech   BlockReason = "hate_speech" // Violates hate speech policies
	BlockReasonSpam         BlockReason = "spam"        // Commercial spam
	BlockReasonSystemPolicy BlockReason = "policy"      // General policy violation
	BlockReasonManual       BlockReason = "manual"      // Admin decision
)

// BlockSeverity defines the severity level of blocked content
type BlockSeverity string

const (
	BlockSeverityCritical BlockSeverity = "critical" // Immediate security/safety threat
	BlockSeverityHigh     BlockSeverity = "high"     // Serious violation requiring action
	BlockSeverityMedium   BlockSeverity = "medium"   // Clear violation
	BlockSeverityLow      BlockSeverity = "low"      // Minor or edge-case violation
)

// BlockAction defines what happens when blocked content is encountered
type BlockAction string

const (
	BlockActionReject     BlockAction = "reject"     // Prevent upload/sharing completely
	BlockActionQuarantine BlockAction = "quarantine" // Allow but isolate from normal access
	BlockActionWarn       BlockAction = "warn"       // Allow but with warning to user
	BlockActionLog        BlockAction = "log"        // Allow but log for tracking
)

// BlockSource defines where a block originated from
type BlockSource string

const (
	BlockSourceScanner  BlockSource = "scanner"  // Automatic scanner detection
	BlockSourceReport   BlockSource = "report"   // User report
	BlockSourceAdmin    BlockSource = "admin"    // Admin decision
	BlockSourceExternal BlockSource = "external" // External block list
)

// Validate performs validation checks on the model
func (b *BlockList) Validate() error {
	// Validation logic can be added here
	return nil
}

// BeforeCreate runs validation before creating a record
func (b *BlockList) BeforeCreate(tx *gorm.DB) error {
	return b.Validate()
}

// BeforeUpdate runs validation before updating a record
func (b *BlockList) BeforeUpdate(tx *gorm.DB) error {
	return b.Validate()
}
