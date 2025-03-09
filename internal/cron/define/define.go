package define

// Task name constants
const CronTaskScanName = "AbuseScan"

// CronTaskScanArgs defines arguments for the scan task
type CronTaskScanArgs struct {
	CaseScanID uint `json:"case_scan_id"`
	SubjectID  uint `json:"subject_id"`
	RequestID  uint `json:"workflow_id"`
}

// CronTaskScanArgsFactory creates a new scan args struct
func CronTaskScanArgsFactory() any {
	return &CronTaskScanArgs{}
}
