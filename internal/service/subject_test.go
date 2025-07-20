package service

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/portal-plugin-abuse/internal/db"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"go.lumeweb.com/queryutil"
	"gorm.io/gorm"
	"testing"

	mh "github.com/multiformats/go-multihash"
)

func TestSubjectService_Create(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		subjectService := core.GetService[typesSvc.SubjectService](ctx, typesSvc.SUBJECT_SERVICE)
		assert.NotNil(tb, subjectService)

		subject := &models.Subject{
			Identifier: []byte("testhash"),
			Type:       models.SubjectTypeHash,
		}

		// Act
		createdSubject, err := subjectService.Create(subject)

		// Assert
		require.NoError(tb, err)
		assert.NotNil(tb, createdSubject)
		assert.Equal(tb, subject.Identifier, createdSubject.Identifier)
		assert.Equal(tb, subject.Type, createdSubject.Type)

		// Verify the subject exists in the database
		var retrievedSubject models.Subject
		err = ctx.DB().First(&retrievedSubject, createdSubject.ID).Error
		require.NoError(tb, err)

		assert.Equal(tb, subject.Identifier, retrievedSubject.Identifier)
		assert.Equal(tb, subject.Type, retrievedSubject.Type)
	},
		coreTesting.WithService(typesSvc.SUBJECT_SERVICE, NewSubjectService),
	)
}

func TestSubjectService_GetByID(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		subjectService := core.GetService[typesSvc.SubjectService](ctx, typesSvc.SUBJECT_SERVICE)
		assert.NotNil(tb, subjectService)

		subjectID := uint(1)
		expectedSubject := &models.Subject{
			Model:      gorm.Model{ID: subjectID},
			Identifier: []byte("testhash"),
			Type:       models.SubjectTypeHash,
		}

		// Manually add the data
		err := ctx.DB().Create(expectedSubject).Error
		require.NoError(tb, err)

		// Act
		retrievedSubject, err := subjectService.GetByID(subjectID)

		// Assert
		require.NoError(tb, err)
		assert.NotNil(tb, retrievedSubject)
		assert.Equal(tb, expectedSubject.ID, retrievedSubject.ID)
		assert.Equal(tb, expectedSubject.Identifier, retrievedSubject.Identifier)
		assert.Equal(tb, expectedSubject.Type, retrievedSubject.Type)
	},
		coreTesting.WithService(typesSvc.SUBJECT_SERVICE, NewSubjectService),
	)
}

func TestSubjectService_GetByID_NotFound(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		subjectService := core.GetService[typesSvc.SubjectService](ctx, typesSvc.SUBJECT_SERVICE)
		assert.NotNil(tb, subjectService)

		subjectID := uint(1)

		// Act
		retrievedSubject, err := subjectService.GetByID(subjectID)

		// Assert
		assert.Error(tb, err)
		assert.ErrorIs(tb, err, db.ErrRecordNotFound)
		assert.Nil(tb, retrievedSubject)
	},
		coreTesting.WithService(typesSvc.SUBJECT_SERVICE, NewSubjectService),
	)
}

func TestSubjectService_FindOrCreate_Existing(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		subjectService := core.GetService[typesSvc.SubjectService](ctx, typesSvc.SUBJECT_SERVICE)
		assert.NotNil(tb, subjectService)

		mh, err := mh.Encode([]byte("testhash"), mh.SHA2_256)
		require.NoError(tb, err)
		identifier := mh
		subjectType := models.SubjectTypeHash

		existingSubject := &models.Subject{
			Identifier: identifier,
			Type:       subjectType,
		}

		// Manually add the data
		err = ctx.DB().Create(existingSubject).Error
		require.NoError(tb, err)

		// Act
		retrievedSubject, err := subjectService.FindOrCreate(core.NewStorageHashFromRawMultihash(mh), subjectType, "")

		// Assert
		require.NoError(tb, err)
		assert.NotNil(tb, retrievedSubject)
		assert.Equal(tb, existingSubject.Identifier, retrievedSubject.Identifier)
		assert.Equal(tb, existingSubject.Type, retrievedSubject.Type)
	},
		coreTesting.WithService(typesSvc.SUBJECT_SERVICE, NewSubjectService),
	)
}

func TestSubjectService_FindOrCreate_New(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		subjectService := core.GetService[typesSvc.SubjectService](ctx, typesSvc.SUBJECT_SERVICE)
		assert.NotNil(tb, subjectService)

		mhb, err := mh.Encode([]byte("testhash"), mh.SHA2_256)
		require.NoError(tb, err)
		_hash := core.NewStorageHashFromMultihashBytes(mhb, 0, nil)
		require.NoError(tb, err)

		identifier := _hash.Multihash()
		subjectType := models.SubjectTypeHash

		// Act
		createdSubject, err := subjectService.FindOrCreate(_hash, subjectType, "")

		// Assert
		require.NoError(tb, err)
		assert.NotNil(tb, createdSubject)
		assert.Equal(tb, identifier, createdSubject.Identifier)
		assert.Equal(tb, subjectType, createdSubject.Type)

		// Verify the subject exists in the database
		var retrievedSubject models.Subject
		err = ctx.DB().Where("identifier = ?", identifier).First(&retrievedSubject).Error
		require.NoError(tb, err)

		assert.Equal(tb, identifier, retrievedSubject.Identifier)
		assert.Equal(tb, subjectType, retrievedSubject.Type)
	},
		coreTesting.WithService(typesSvc.SUBJECT_SERVICE, NewSubjectService),
	)
}

func TestSubjectService_FindOrCreateByURL_New(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		subjectService := core.GetService[typesSvc.SubjectService](ctx, typesSvc.SUBJECT_SERVICE)
		assert.NotNil(tb, subjectService)

		testURL := "https://example.com/test"
		subjectType := models.SubjectTypeURL

		// Act
		createdSubject, err := subjectService.FindOrCreateByURL(testURL, subjectType)

		// Assert
		require.NoError(tb, err)
		assert.NotNil(tb, createdSubject)
		assert.Equal(tb, testURL, createdSubject.SourceURL)
		assert.Equal(tb, subjectType, createdSubject.Type)

		// Verify the subject exists in the database
		var retrievedSubject models.Subject
		err = ctx.DB().Where("source_url = ?", testURL).First(&retrievedSubject).Error
		require.NoError(tb, err)

		assert.Equal(tb, testURL, retrievedSubject.SourceURL)
		assert.Equal(tb, subjectType, retrievedSubject.Type)
	},
		coreTesting.WithService(typesSvc.SUBJECT_SERVICE, NewSubjectService),
	)
}

func TestSubjectService_List(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		subjectService := core.GetService[typesSvc.SubjectService](ctx, typesSvc.SUBJECT_SERVICE)
		assert.NotNil(tb, subjectService)

		subject1 := &models.Subject{
			Identifier: []byte("testhash1"),
			Type:       models.SubjectTypeHash,
		}
		subject2 := &models.Subject{
			Identifier: []byte("testhash2"),
			Type:       models.SubjectTypeURL,
		}

		// Manually add the data
		err := ctx.DB().Create(subject1).Error
		require.NoError(tb, err)
		err = ctx.DB().Create(subject2).Error
		require.NoError(tb, err)

		// Act
		filters := []queryutil.CrudFilter{}
		sorts := []queryutil.Sort{}
		pagination := queryutil.DefaultPagination
		retrievedSubjects, total, err := subjectService.List(filters, sorts, pagination)

		// Assert
		require.NoError(tb, err)
		assert.Equal(tb, int64(2), total)
		assert.Len(tb, retrievedSubjects, 2)
		assert.Equal(tb, subject1.Identifier, retrievedSubjects[0].Identifier)
		assert.Equal(tb, subject2.Identifier, retrievedSubjects[1].Identifier)
	},
		coreTesting.WithService(typesSvc.SUBJECT_SERVICE, NewSubjectService),
	)
}
