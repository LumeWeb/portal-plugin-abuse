package workflow

const (
	AbuseScanWorkflowName = "abuse.scan"
)

type AbuseScanWorkflowData struct {
	CaseScanID uint `json:"case_scan_id"`
	SubjectID  uint `json:"subject_id"`
}
