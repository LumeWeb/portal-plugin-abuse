package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/portal-plugin-abuse/internal/api/dto"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal-plugin-abuse/internal/service/mocks"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"gorm.io/gorm"
)

func TestCreateSubject_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		mockSubjectSvc := core.GetService[*mocks.MockSubjectService](ctx, typesSvc.SUBJECT_SERVICE)

		hash, err := core.ParseStorageHash(exampleCID)
		require.NoError(tb, err)

		mockSubject := &models.Subject{
			Model:      gorm.Model{ID: 1},
			Identifier: hash.Multihash(),
			Type:       models.SubjectTypeHash,
		}

		mockSubjectSvc.EXPECT().Create(mock.AnythingOfType("*models.Subject")).Return(mockSubject, nil).Once()

		reqBody := dto.SubjectCreateRequest{
			Identifier: exampleCID,
			Type:       string(models.SubjectTypeHash),
		}
		body, err := json.Marshal(reqBody)
		require.NoError(tb, err)

		req := ctx.NewAPIRequest(http.MethodPost, "/api/abuse/subjects", body)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		assert.Equal(tb, http.StatusCreated, w.Code)

		var response dto.SubjectResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Equal(tb, exampleCID, response.Identifier)

	})
}

func TestCreateSubject_ValidationFailure(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		reqBody := `{"type": "invalid"}`

		req := ctx.NewAPIRequest(http.MethodPost, "/api/abuse/subjects", []byte(reqBody))
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		assert.Equal(tb, http.StatusUnprocessableEntity, w.Code)
	})
}

func TestGetSubject_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		mockSubjectSvc := core.GetService[*mocks.MockSubjectService](ctx, typesSvc.SUBJECT_SERVICE)

		hash, err := core.ParseStorageHash(exampleCID)
		require.NoError(tb, err)

		mockSubject := &models.Subject{
			Model:      gorm.Model{ID: 1},
			Identifier: hash.Multihash(),
			Type:       models.SubjectTypeURL,
		}
		mockSubjectSvc.EXPECT().GetByID(uint(1)).Return(mockSubject, nil).Once()

		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/subjects/1", nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		assert.Equal(tb, http.StatusOK, w.Code)

		var response dto.SubjectResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Equal(tb, exampleCID, response.Identifier)
	})
}

func TestGetSubject_NotFound(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		mockSubjectSvc := core.GetService[*mocks.MockSubjectService](ctx, typesSvc.SUBJECT_SERVICE)

		mockSubjectSvc.EXPECT().GetByID(uint(999)).Return(nil, gorm.ErrRecordNotFound).Once()

		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/subjects/999", nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		assert.Equal(tb, http.StatusNotFound, w.Code)
	})
}

func TestListSubjects_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		mockSubjectSvc := core.GetService[*mocks.MockSubjectService](ctx, typesSvc.SUBJECT_SERVICE)

		hash, err := core.ParseStorageHash(exampleCID)
		require.NoError(tb, err)

		mockSubjects := []models.Subject{
			{Model: gorm.Model{ID: 1}, Identifier: hash.Multihash()},
			{Model: gorm.Model{ID: 2}, Identifier: hash.Multihash()},
		}
		mockSubjectSvc.EXPECT().List(mock.Anything, mock.Anything, mock.Anything).Return(mockSubjects, int64(2), nil).Once()

		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/subjects", nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		assert.Equal(tb, http.StatusOK, w.Code)

		var response struct {
			Data []dto.SubjectResponse `json:"data"`
		}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Len(tb, response.Data, 2)
		assert.Equal(tb, exampleCID, response.Data[0].Identifier)
	})
}

func TestListSubjects_ServiceError(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		mockSubjectSvc := core.GetService[*mocks.MockSubjectService](ctx, typesSvc.SUBJECT_SERVICE)

		mockSubjectSvc.EXPECT().List(mock.Anything, mock.Anything, mock.Anything).Return(nil, int64(0), fmt.Errorf("service error")).Once()

		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/subjects", nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		assert.Equal(tb, http.StatusInternalServerError, w.Code)
	})
}
