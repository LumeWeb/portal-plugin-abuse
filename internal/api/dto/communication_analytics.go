package dto

import (
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
)

// CommunicationTimelineItem represents a single entry in the communication timeline.
type CommunicationTimelineItem struct {
	HourlyInterval string `json:"hourly_interval"`
	CommCount      int64  `json:"count"`
}

var _ httputil.DTOResponse[[]models.CommunicationHourlyCount] = (*CommunicationTimelineResponse)(nil)

// CommunicationTimelineResponse represents the response for the communication timeline endpoint.
type CommunicationTimelineResponse struct {
	Items []CommunicationTimelineItem `json:"items"`
}

// FromModel converts a slice of CommunicationHourlyCount models to a CommunicationTimelineResponse DTO.
func (r *CommunicationTimelineResponse) FromModel(counts []models.CommunicationHourlyCount) error {
	r.Items = make([]CommunicationTimelineItem, len(counts))
	for i, count := range counts {
		r.Items[i] = CommunicationTimelineItem{
			HourlyInterval: count.HourlyInterval,
			CommCount:      count.CommCount,
		}
	}
	return nil
}
