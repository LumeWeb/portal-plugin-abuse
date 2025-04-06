package dto

import (
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-plugin-abuse/internal/types/service"
)

var _ httputil.DTOResponse[[]service.BlockReasonCount] = (*BlockReasonListResponse)(nil)

// BlockReasonCountResponse represents a single block reason count.
type BlockReasonCountResponse struct {
	BlockDate   string `json:"block_date"`
	BlockReason string `json:"reason"` // String representation of BlockReason
	BlockCount  int64  `json:"count"`
}

// BlockReasonListResponse represents a list of block reason counts.
type BlockReasonListResponse struct {
	Items []BlockReasonCountResponse `json:"items"`
}

// FromModel converts a slice of service.BlockReasonCount to populate the DTO.
func (r *BlockReasonListResponse) FromModel(counts []service.BlockReasonCount) error {
	r.Items = make([]BlockReasonCountResponse, len(counts))
	for i, count := range counts {
		r.Items[i] = BlockReasonCountResponse{
			BlockDate:   count.BlockDate,
			BlockReason: count.BlockReason,
			BlockCount:  count.BlockCount,
		}
	}
	return nil
}
