package dto

import (
	"encoding/json"
	"fmt"
	z "github.com/Oudwins/zog"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal/core"
	"gorm.io/datatypes"
	"time"
)

var _ httputil.DTOValidator = (*ManualScanRequest)(nil)
var _ httputil.DTOResponse[*models.CaseScan] = (*ScanResponse)(nil)
var _ httputil.DTOResponse[*core.ScanResult] = (*ScanResultResponse)(nil)

// ScanResponse represents a scan operation response
type ScanResponse struct {
	ID          uint       `json:"id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CaseID      uint       `json:"case_id"`
	SubjectID   uint       `json:"subject_id"`
	Status      string     `json:"status"`
	ScheduledAt time.Time  `json:"scheduled_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// FromModel converts a CaseScan model to a ScanResponse
func (r *ScanResponse) FromModel(scan *models.CaseScan) error {
	r.ID = scan.ID
	r.CreatedAt = scan.CreatedAt
	r.UpdatedAt = scan.UpdatedAt
	r.CaseID = scan.CaseID
	r.SubjectID = scan.SubjectID
	r.Status = string(scan.Status)
	r.ScheduledAt = scan.ScheduledFor
	return nil
}

// ScanResultResponse represents the results of a scan
// ScanResultResponse represents the results of a scan
type ScanResultResponse struct {
	Passed    bool           `json:"passed"`
	Reason    string         `json:"reason,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
	ScannerID string         `json:"scanner_id"`
	Metadata  datatypes.JSON `json:"metadata,omitempty"`
}

// FromModel converts a core.ScanResult to a ScanResultResponse
func (r *ScanResultResponse) FromModel(result *core.ScanResult) error {
	r.Passed = result.Passed
	r.Reason = result.Reason
	r.Timestamp = result.CreatedAt
	r.ScannerID = result.ScannerID

	// Convert the metadata map to datatypes.JSON
	metadataJSON, err := json.Marshal(result.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	r.Metadata = metadataJSON

	return nil
}

type ManualScanRequest struct {
	CaseID int `json:"case_id"`
}

func (r *ManualScanRequest) ToModel() (*models.CaseScan, error) {
	return &models.CaseScan{
		CaseID: uint(r.CaseID),
	}, nil
}

func (r *ManualScanRequest) Schema() *z.StructSchema {
	return z.Struct(z.Schema{
		"CaseID": z.Int().Required().GT(0),
	})
}
