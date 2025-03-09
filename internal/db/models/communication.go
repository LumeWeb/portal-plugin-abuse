package models

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// Communication represents a message related to a case
type Communication struct {
	gorm.Model
	CaseID    uint
	Case      Case
	SenderID  uint // User or Reporter ID
	Type      CommunicationType
	Direction CommunicationDirection
	Content   string
	ThreadID  string // For email threading
	ParentID  *uint  // For hierarchical threading
}

// TableName specifies the database table name
func (Communication) TableName() string {
	return "abuse_communications"
}

// Validate performs validation on the communication
func (c *Communication) Validate() error {
	// Content is required
	if strings.TrimSpace(c.Content) == "" {
		return fmt.Errorf("communication content is required")
	}

	// Check valid types
	validTypes := map[CommunicationType]bool{
		CommunicationTypeEmail:    true,
		CommunicationTypeNote:     true,
		CommunicationTypeResponse: true,
	}
	if !validTypes[c.Type] {
		return fmt.Errorf("invalid communication type: %s", c.Type)
	}

	// Check valid directions
	validDirections := map[CommunicationDirection]bool{
		CommunicationDirectionIncoming: true,
		CommunicationDirectionOutgoing: true,
		CommunicationDirectionInternal: true,
		CommunicationDirectionExternal: true,
	}
	if !validDirections[c.Direction] {
		return fmt.Errorf("invalid communication direction: %s", c.Direction)
	}

	return nil
}

// BeforeCreate validates the communication before creation
func (c *Communication) BeforeCreate(tx *gorm.DB) error {
	return c.Validate()
}

// BeforeUpdate validates the communication before update
func (c *Communication) BeforeUpdate(tx *gorm.DB) error {
	return c.Validate()
}
