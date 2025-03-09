package models

import (
	coreModel "go.lumeweb.com/portal/db/models"
	"gorm.io/gorm"
)

// Reporter represents someone who submits abuse reports
type Reporter struct {
	gorm.Model
	Email  string
	Name   string
	UserID *string
	User   *coreModel.User `gorm:"-"` // Portal user reference (not stored)
}

