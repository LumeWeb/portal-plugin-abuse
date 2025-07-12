package models

import (
	"errors"
	coreModel "go.lumeweb.com/portal/db/models"
	emailverifier "github.com/AfterShip/email-verifier"
	"gorm.io/gorm"
)

var (
	emailVerifier = newEmailVerifier()
)

func newEmailVerifier() *emailverifier.Verifier {
	verifier := emailverifier.NewVerifier()
	verifier.DisableSMTPCheck()
	verifier.DisableGravatarCheck()
	verifier.DisableDomainSuggest()
	verifier.DisableAutoUpdateDisposable()
	return verifier
}

var (
	ErrInvalidReporterEmail = errors.New("invalid reporter email")
	ErrReporterNotTrusted   = errors.New("reporter not trusted")
)

// Reporter represents someone who submits abuse reports
type ReporterTrustStatus int

const (
	ReporterNew ReporterTrustStatus = iota
	ReporterUntrusted
	ReporterTrusted
)
type Reporter struct {
	gorm.Model
	Email  string `gorm:"not null"`
	Name   string `gorm:"not null"`
	UserID *string
	User   *coreModel.User `gorm:"-"` // Portal user reference (not stored)
}

func (Reporter) TableName() string {
	return "abuse_reporters"
}

// Validate performs validation on the reporter model
func (r *Reporter) Validate() error {
	if r.Email == "" {
		return ErrInvalidReporterEmail
	}

	verify, err := emailVerifier.Verify(r.Email)
	if err != nil {
		return ErrInvalidReporterEmail
	}
	if !verify.Syntax.Valid {
		return ErrInvalidReporterEmail
	}

	if r.Name == "" {
		return errors.New("reporter name is required")
	}
	return nil
}

// BeforeCreate validates the reporter before creation
func (r *Reporter) BeforeCreate(tx *gorm.DB) error {
	return r.Validate()
}

// BeforeUpdate validates the reporter before update
func (r *Reporter) BeforeUpdate(tx *gorm.DB) error {
	return r.Validate()
}

