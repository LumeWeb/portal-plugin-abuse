package models

import (
	"github.com/samber/lo"
)

// CaseType defines the type of abuse case
type CaseType string

const (
	CaseTypeSpam                   CaseType = "spam"
	CaseTypeHarassment             CaseType = "harassment"
	CaseTypeMalware                CaseType = "malware"
	CaseTypePhishing               CaseType = "phishing"
	CaseTypeCopyrightViolation     CaseType = "copyright_violation"
	CaseTypeResourceAbuse          CaseType = "resource_abuse"
	CaseTypeIllegalOrHarmfulContent CaseType = "illegal_or_harmful_content"
	CaseTypeOther                  CaseType = "other"
)

// ValidCaseTypes contains all valid CaseType values for validation
var ValidCaseTypes = []string{
	string(CaseTypeSpam),
	string(CaseTypeHarassment),
	string(CaseTypeMalware),
	string(CaseTypePhishing),
	string(CaseTypeCopyrightViolation),
	string(CaseTypeResourceAbuse),
	string(CaseTypeIllegalOrHarmfulContent),
	string(CaseTypeOther),
}

// ValidCasePriorities contains all valid CasePriority values
var ValidCasePriorities = []string{
	string(CasePriorityLow),
	string(CasePriorityMedium),
	string(CasePriorityHigh),
	string(CasePriorityCritical),
}

// ValidCasePriorityMap provides O(1) lookups for valid priorities
var ValidCasePriorityMap = lo.SliceToMap(ValidCasePriorities, func(s string) (CasePriority, bool) {
	return CasePriority(s), true
})

// ValidCaseStatuses contains all valid CaseStatus values
var ValidCaseStatuses = []string{
	string(CaseStatusNew),
	string(CaseStatusInProgress),
	string(CaseStatusResolved),
	string(CaseStatusClosed),
}

// ValidCaseStatusMap provides O(1) lookups for valid statuses
var ValidCaseStatusMap = lo.SliceToMap(ValidCaseStatuses, func(s string) (CaseStatus, bool) {
	return CaseStatus(s), true
})

// ValidReportSources contains all valid ReportSource values
var ValidReportSources = []string{
	string(ReportSourceWebForm),
	string(ReportSourceEmail),
	string(ReportSourceAPI),
}

// CaseStatus defines the current status of a case
type CaseStatus string

const (
	CaseStatusNew        CaseStatus = "new"
	CaseStatusInProgress CaseStatus = "in_progress"
	CaseStatusResolved   CaseStatus = "resolved"
	CaseStatusClosed     CaseStatus = "closed"
)

// CasePriority defines the priority level of a case
type CasePriority string

const (
	CasePriorityLow      CasePriority = "low"
	CasePriorityMedium   CasePriority = "medium"
	CasePriorityHigh     CasePriority = "high"
	CasePriorityCritical CasePriority = "critical"
)

// SubjectType defines the type of entity being reported
type SubjectType string

const (
	SubjectTypeHash SubjectType = "hash" // All subjects are content hashes
	SubjectTypeURL  SubjectType = "url"  // URL type for location references
)

// ReportSource defines where the report originated from
type ReportSource string

const (
	ReportSourceWebForm ReportSource = "web_form"
	ReportSourceEmail   ReportSource = "email"
	ReportSourceAPI     ReportSource = "api"
)

// ScanStatus defines the status of a content scan
type ScanStatus string

const (
	ScanStatusPending  ScanStatus = "pending"
	ScanStatusScanning ScanStatus = "scanning"
	ScanStatusClean    ScanStatus = "clean"
	ScanStatusFlagged  ScanStatus = "flagged"
	ScanStatusError    ScanStatus = "error"
)

// CommunicationType defines the type of communication
type CommunicationType string

const (
	CommunicationTypeEmail    CommunicationType = "email"
	CommunicationTypeNote     CommunicationType = "note"
	CommunicationTypeResponse CommunicationType = "response"
)

// CommunicationDirection defines the direction of communication
type CommunicationDirection string

const (
	CommunicationDirectionIncoming CommunicationDirection = "incoming"
	CommunicationDirectionOutgoing CommunicationDirection = "outgoing"
	CommunicationDirectionInternal CommunicationDirection = "internal"
	CommunicationDirectionExternal CommunicationDirection = "external"
)
