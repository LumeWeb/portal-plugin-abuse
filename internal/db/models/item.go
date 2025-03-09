// Package models defines the database models used by the template plugin
package models

import (
	"gorm.io/gorm"
)

// Item represents a basic item in the system
// It demonstrates a simple GORM model with basic fields
type Item struct {
	gorm.Model          // Provides ID, CreatedAt, UpdatedAt, DeletedAt fields
	Name        string  `json:"name" gorm:"not null"`         // Required name field
	Description string  `json:"description" gorm:"type:text"` // Optional description field
}
