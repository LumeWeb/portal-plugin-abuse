package models

// CaseType defines the type of abuse case
type CaseType string

const (
	CaseTypeSpam       CaseType = "spam"
	CaseTypeHarassment CaseType = "harassment"
	CaseTypeContent    CaseType = "content"
	CaseTypeMalware    CaseType = "malware" // Malware, viruses, and security threats
	CaseTypeOther      CaseType = "other"
)

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
