package models

import (
	mh "github.com/multiformats/go-multihash"
	"gorm.io/gorm"
)

// Subject represents the content being reported for abuse
type Subject struct {
	gorm.Model
	Identifier mh.Multihash
	Type       SubjectType
	SourceURL  string // Original URL if reported via URL (optional)
}

func (Subject) TableName() string {
	return "abuse_subjects"
}
