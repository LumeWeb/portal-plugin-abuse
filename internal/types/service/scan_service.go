package service

import (
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/queryutil"
)

type ScanService interface {
	core.Service
	core.Configurable

	// CreateScanRequest creates a scan request for an existing subject
	CreateScanRequest(caseID uint) error

	// GetScansForCase gets all scans for a case
	GetScansForCase(caseID uint, pagination queryutil.Pagination) ([]models.CaseScan, int64, error)

	// GetScanById gets scan
	GetScanById(scanID uint) (*models.CaseScan, error)

	// SaveScanResults saves the results of a scan for the given scan ID and returns an error if the operation fails.
	SaveScanResults(scanID uint, results []*core.ScanResult) error

	// GetScanResults retrieves the scan results
	GetScanResults(scanID uint) ([]*core.ScanResult, error)

	// UpdateScanStatus updates the status of a scan
	UpdateScanStatus(scanID uint, status models.ScanStatus) error
}
