package db

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"go.lumeweb.com/queryutil"

	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/portal/db"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Database error types
var (
	ErrRecordNotFound      = gorm.ErrRecordNotFound
	ErrDuplicateEntry      = errors.New("duplicate entry")
	ErrConstraintViolation = errors.New("constraint violation")
	ErrTransactionFailed   = errors.New("transaction failed")
	ErrInvalidID           = errors.New("invalid ID")
	ErrInvalidInput        = errors.New("invalid input")
)

// DBError wraps database errors with additional context
type DBError struct {
	Err         error
	Operation   string
	Entity      string
	ID          uint
	Constraints []string
}

func (e *DBError) Error() string {
	return fmt.Sprintf("db error: %s (entity: %s, operation: %s)", e.Err.Error(), e.Entity, e.Operation)
}

func (e *DBError) Unwrap() error {
	return e.Err
}

// HandleDBError handles and wraps database errors
func HandleDBError(err error, operation, entity string, id uint) error {
	if err == nil {
		return nil
	}

	dbErr := &DBError{
		Err:       err,
		Operation: operation,
		Entity:    entity,
		ID:        id,
	}

	// Handle specific GORM errors
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrRecordNotFound
	}

	// Handle MySQL/SQLite specific errors using error message parsing
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "duplicate entry"):
		return ErrDuplicateEntry
	case strings.Contains(msg, "foreign key constraint"):
		dbErr.Constraints = extractConstraints(msg)
		return ErrConstraintViolation
	case strings.Contains(msg, "constraint"):
		dbErr.Constraints = extractConstraints(msg)
		return ErrConstraintViolation
	case strings.Contains(msg, "transaction"):
		return ErrTransactionFailed
	}

	return dbErr
}

func extractConstraints(errorMsg string) []string {
	constraints := make([]string, 0)
	// Add logic to extract constraint names from error message if needed
	return constraints
}

// Error helpers
func IsRecordNotFound(err error) bool {
	return errors.Is(err, ErrRecordNotFound)
}

func IsDuplicateEntry(err error) bool {
	return errors.Is(err, ErrDuplicateEntry)
}

func IsConstraintViolation(err error) bool {
	return errors.Is(err, ErrConstraintViolation)
}

func IsTransactionFailed(err error) bool {
	return errors.Is(err, ErrTransactionFailed)
}

func GetDBError(err error) (*DBError, bool) {
	var dbErr *DBError
	if errors.As(err, &dbErr) {
		return dbErr, true
	}
	return nil, false
}

// handleError centralizes error logging and wrapping.
func handleError(_ core.Context, logger *core.Logger, operation string, err error, fields ...zap.Field) error {
	if err != nil {
		logger.Error(operation+" failed", append(fields, zap.Error(err))...)
		return fmt.Errorf("%s operation failed: %w", operation, err)
	}
	return nil
}

type TransactionFunc func(tx *gorm.DB) error

type DBOption func(*gorm.DB) *gorm.DB

func applyDbOptions(_db *gorm.DB, opts []DBOption) *gorm.DB {
	lastDb := _db
	for _, opt := range opts {
		latestDb := opt(_db)
		if latestDb == nil {
			continue
		}

		lastDb = latestDb
	}

	return lastDb
}

func WithDBPreload(preloads ...string) DBOption {
	return func(_db *gorm.DB) *gorm.DB {
		for _, preload := range preloads {
			_db = _db.Preload(preload)
		}
		return _db
	}
}

func WithDBGroupBy(groupBy string) DBOption {
	return func(_db *gorm.DB) *gorm.DB {
		return _db.Group(groupBy)
	}
}

func WithDBSelect(selectClause string) DBOption {
	return func(_db *gorm.DB) *gorm.DB {
		return _db.Select(selectClause)
	}
}

func ExecuteTransaction(ctx context.Context, coreCtx core.Context, _db *gorm.DB, operation string, txFunc TransactionFunc, fields ...zap.Field) error {
	logger := coreCtx.Logger()

	err := db.RetryableTransaction(coreCtx, _db, func(tx *gorm.DB) *gorm.DB {
		tx = tx.WithContext(ctx) // Apply standard Go context

		if err := txFunc(tx); err != nil {
			logger.Error(operation+" failed", append(fields, zap.Error(err))...)
			_ = tx.AddError(err) // Let the transaction framework handle the error
		}
		return tx
	})

	return handleError(coreCtx, logger, operation, err, fields...)
}

// ApplySearchQuery applies a LIKE search query to the GORM DB instance based on the "searchable" tag.
func ApplySearchQuery[T any](tx *gorm.DB, query string) *gorm.DB {
	if query == "" {
		return tx
	}

	var model T
	modelType := reflect.TypeOf(model)

	// If model is a pointer, get the underlying type
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	// Iterate over the fields of the model
	var searchableColumns []string
	for i := 0; i < modelType.NumField(); i++ {
		field := modelType.Field(i)
		tag := field.Tag.Get("gorm")
		if strings.Contains(tag, "searchable") {
			// Add the field name to the list of searchable columns
			searchableColumns = append(searchableColumns, field.Name)
		}
	}

	// Build the where clause with OR conditions for each searchable column
	whereClause := ""
	for i, column := range searchableColumns {
		if i > 0 {
			whereClause += " OR "
		}
		whereClause += fmt.Sprintf("%s LIKE ?", column)
	}

	// Prepare values for the where clause
	values := make([]interface{}, len(searchableColumns))
	for i := range searchableColumns {
		values[i] = "%" + query + "%"
	}

	// Apply the where clause to the query
	return tx.Where(whereClause, values...)
}

// ListOption defines a function that modifies the GORM DB instance.
type ListOption = DBOption

// WithSearchQuery returns a ListOption that applies the search query.
func WithSearchQuery[T any](query string) ListOption {
	return func(tx *gorm.DB) *gorm.DB {
		if query != "" {
			return ApplySearchQuery[T](tx, query)
		}
		return tx
	}
}

// Create creates a new record in the database.
func Create[T any](ctx context.Context, coreCtx core.Context, _db *gorm.DB, record *T, options ...DBOption) error {
	_db = applyDbOptions(_db, options)
	return ExecuteTransaction(ctx, coreCtx, _db, "Create", func(tx *gorm.DB) error {
		err := tx.Create(record).Error
		if err != nil {
			var entity T
			return HandleDBError(err, "Create", fmt.Sprintf("%T", entity), 0)
		}
		return nil
	})
}

// GetByID retrieves a record from the database by its ID.
func GetByID[T any](ctx context.Context, coreCtx core.Context, _db *gorm.DB, id uint, record *T, options ...DBOption) error {
	return ExecuteTransaction(ctx, coreCtx, _db, "GetByID", func(tx *gorm.DB) error {
		_db = applyDbOptions(_db, options)
		err := tx.First(record, id).Error
		if err != nil {
			var entity T
			return HandleDBError(err, "GetByID", fmt.Sprintf("%T", entity), id)
		}
		return nil
	}, zap.Uint("id", id))
}

// GetByProperty retrieves a record from the database by a given property
func GetByProperty[T any](ctx context.Context, coreCtx core.Context, _db *gorm.DB, property string, value any, record *T, options ...DBOption) error {
	return ExecuteTransaction(ctx, coreCtx, _db, "GetByProperty", func(tx *gorm.DB) error {
		_db = applyDbOptions(_db, options)
		err := tx.Model(record).Where(property+" = ?", value).First(record).Error
		if err != nil {
			var entity T
			return HandleDBError(err, "GetByProperty", fmt.Sprintf("%T", entity), 0)
		}
		return nil
	}, zap.String("property", property), zap.Any("value", value))
}

// GetByProperties retrieves a record from the database by multiple properties.
func GetByProperties[T any](ctx context.Context, coreCtx core.Context, _db *gorm.DB, properties map[string]any, record *T, options ...DBOption) error {
	return ExecuteTransaction(ctx, coreCtx, _db, "GetByProperties", func(tx *gorm.DB) error {
		_db = applyDbOptions(_db, options)
		_db = tx.Model(record)
		for property, value := range properties {
			if value == nil {
				_db = _db.Where(fmt.Sprintf("%s IS NULL", property))
			} else {
				_db = _db.Where(property+" = ?", value)
			}
		}

		err := _db.First(record).Error
		if err != nil {
			var entity T
			return HandleDBError(err, "GetByProperties", fmt.Sprintf("%T", entity), 0)
		}
		return nil
	}, zap.Any("properties", properties))
}

// Update updates a record in the database.
func Update[T any](ctx context.Context, coreCtx core.Context, _db *gorm.DB, record *T, options ...DBOption) error {
	return ExecuteTransaction(ctx, coreCtx, _db, "Update", func(tx *gorm.DB) error {
		_db = applyDbOptions(_db, options)
		return tx.Save(record).Error
	})
}

// Delete soft deletes a record from the database.
func Delete[T any](ctx context.Context, coreCtx core.Context, _db *gorm.DB, id uint, record *T, options ...DBOption) error {
	return ExecuteTransaction(ctx, coreCtx, _db, "Delete", func(tx *gorm.DB) error {
		_db = applyDbOptions(_db, options)
		//GORM will automatically set the DeletedAt timestamp.
		return tx.Delete(record, id).Error
	}, zap.Uint("id", id))
}

// DeleteByProperty soft deletes a record by a property value
func DeleteByProperty[T any](ctx context.Context, coreCtx core.Context, _db *gorm.DB, property string, value any, record *T, options ...DBOption) error {
	return ExecuteTransaction(ctx, coreCtx, _db, "DeleteByProperty", func(tx *gorm.DB) error {
		_db = applyDbOptions(_db, options)
		//GORM will automatically set the DeletedAt timestamp.
		err := tx.Model(record).Where(property+" = ?", value).Delete(record).Error
		if err != nil {
			var entity T
			return HandleDBError(err, "DeleteByProperty", fmt.Sprintf("%T", entity), 0)
		}
		return nil
	}, zap.String("property", property), zap.Any("value", value))
}

// HardDelete permanently removes a record from the database.
func HardDelete[T any](ctx context.Context, coreCtx core.Context, _db *gorm.DB, id uint, record *T, options ...DBOption) error {
	return ExecuteTransaction(ctx, coreCtx, _db, "HardDelete", func(tx *gorm.DB) error {
		_db = applyDbOptions(_db, options)
		return tx.Unscoped().Delete(record, id).Error // Use Unscoped() for hard delete
	}, zap.Uint("id", id))
}

// Undelete restores a soft-deleted record.
func Undelete[T any](ctx context.Context, coreCtx core.Context, _db *gorm.DB, id uint, record *T, options ...DBOption) error {
	return ExecuteTransaction(ctx, coreCtx, _db, "Undelete", func(tx *gorm.DB) error {
		_db = applyDbOptions(_db, options)
		err := tx.Unscoped().First(record, id).Error
		if err != nil {
			return err
		}
		// Set DeletedAt to nil to undelete
		return tx.Model(record).Update("deleted_at", nil).Error
	}, zap.Uint("id", id))
}

// List retrieves a list of records from the database with filtering, sorting, and pagination.
func List[T any](ctx context.Context, coreCtx core.Context, _db *gorm.DB, filters []queryutil.CrudFilter, sorts []queryutil.Sort, pagination queryutil.Pagination, records *[]T, total *int64, options ...ListOption) error {
	return ExecuteTransaction(ctx, coreCtx, _db, "List", func(tx *gorm.DB) error {
		// Apply options
		for _, option := range options {
			tx = option(tx)
		}

		tx = queryutil.ApplyFilters(tx, filters, nil)
		tx = queryutil.ApplySort(tx, sorts)

		var count int64
		if err := tx.Model(new(T)).Count(&count).Error; err != nil {
			return err
		}
		*total = count

		pagingNotSet := pagination.GetOffset() == 0 && pagination.GetLimit() == 0

		if !pagingNotSet {
			tx = tx.Offset(pagination.GetOffset()).Limit(pagination.GetLimit())
		}

		return tx.Find(records).Error
	})
}

// ListAggregate retrieves aggregated data without model-based counting
func ListAggregate[T any](ctx context.Context, coreCtx core.Context, _db *gorm.DB, filters []queryutil.CrudFilter, sorts []queryutil.Sort, pagination queryutil.Pagination, records *[]T, options ...ListOption) error {
	return ExecuteTransaction(ctx, coreCtx, _db, "ListAggregate", func(tx *gorm.DB) error {
		// Apply options
		for _, option := range options {
			tx = option(tx)
		}

		tx = queryutil.ApplyFilters(tx, filters, nil)
		tx = queryutil.ApplySort(tx, sorts)

		pagingNotSet := pagination.GetOffset() == 0 && pagination.GetLimit() == 0

		if !pagingNotSet {
			tx = tx.Offset(pagination.GetOffset()).Limit(pagination.GetLimit())
		}

		return tx.Find(records).Error
	})
}

// ListIncludingSoftDeleted retrieves a list of records, including soft-deleted records.
func ListIncludingSoftDeleted[T any](ctx context.Context, coreCtx core.Context, _db *gorm.DB, filters []queryutil.CrudFilter, sorts []queryutil.Sort, pagination queryutil.Pagination, records *[]T, total *int64) error {
	return ExecuteTransaction(ctx, coreCtx, _db, "ListIncludingSoftDeleted", func(tx *gorm.DB) error {
		tx = tx.Unscoped() //include soft deleted records
		tx = queryutil.ApplyFilters(tx, filters, nil)
		tx = queryutil.ApplySort(tx, sorts)

		var count int64
		if err := tx.Model(new(T)).Count(&count).Error; err != nil {
			return err
		}
		*total = count

		tx = tx.Offset(pagination.GetOffset()).Limit(pagination.GetLimit())

		return tx.Find(records).Error
	})
}

// BulkCreate creates multiple records in the database.
func BulkCreate[T any](ctx context.Context, coreCtx core.Context, _db *gorm.DB, records []T, options ...DBOption) error {
	return ExecuteTransaction(ctx, coreCtx, _db, "BulkCreate", func(tx *gorm.DB) error {
		_db = applyDbOptions(_db, options)
		return tx.CreateInBatches(records, 100).Error // Batch size of 100
	})
}

// BulkUpdate updates multiple records in the database, using only the specified fields.
func BulkUpdate[T any](ctx context.Context, coreCtx core.Context, _db *gorm.DB, records []T, fields []string, options ...DBOption) error {
	_db = applyDbOptions(_db, options)
	return ExecuteTransaction(ctx, coreCtx, _db, "BulkUpdate", func(tx *gorm.DB) error {
		_db = applyDbOptions(_db, options)
		return tx.Select(fields).Updates(records).Error
	})
}

// Count retrieves the number of records in the database based on the provided filters.
func Count[T any](ctx context.Context, coreCtx core.Context, _db *gorm.DB, filters []queryutil.CrudFilter, options ...DBOption) (int64, error) {
	var count int64
	err := ExecuteTransaction(ctx, coreCtx, _db, "Count", func(tx *gorm.DB) error {
		_db = applyDbOptions(_db, options)
		tx = queryutil.ApplyFilters(tx, filters, nil)
		return tx.Model(new(T)).Count(&count).Error
	})

	return count, err
}
