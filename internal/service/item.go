package service

import (
	"go.lumeweb.com/portal-plugin-template/internal/db/models"
	"go.lumeweb.com/portal/core"
	"gorm.io/gorm"
)

const ITEM_SERVICE = "item"

// Pagination defines the structure for paginated requests
type Pagination struct {
	Page  int // Current page number (1-based)
	Limit int // Number of items per page
}

// ItemService defines the interface for managing items in the system.
// It provides methods for CRUD operations and search functionality.
type ItemService interface {
	core.Service
	ListItems(pagination *Pagination) ([]models.Item, int64, error)
	CreateItem(name string, description string) (*models.Item, error)
	GetItem(id uint64) (*models.Item, error)
	UpdateItem(id uint64, name string, description string) error
	DeleteItem(id uint64) error
	SearchItems(query string, limit int) ([]models.Item, int64, error)
}

// Verify ItemServiceDefault implements ItemService interface
var _ ItemService = (*ItemServiceDefault)(nil)

// ItemServiceDefault provides the default implementation of ItemService
type ItemServiceDefault struct {
	ctx    core.Context
	db     *gorm.DB
	logger *core.Logger
}

func NewItemService() (core.Service, []core.ContextBuilderOption, error) {
	service := &ItemServiceDefault{}

	return service, core.ContextOptions(
		core.ContextWithStartupFunc(func(ctx core.Context) error {
			service.ctx = ctx
			service.db = ctx.DB()
			service.logger = ctx.ServiceLogger(service)
			return nil
		}),
	), nil
}

func (s *ItemServiceDefault) ID() string {
	return ITEM_SERVICE
}

// ListItems retrieves a paginated list of items
// Returns the items for the requested page, total count of all items, and any error
func (s *ItemServiceDefault) ListItems(pagination *Pagination) ([]models.Item, int64, error) {
	var items []models.Item
	var total int64

	if err := s.db.Model(&models.Item{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (pagination.Page - 1) * pagination.Limit
	if err := s.db.Offset(offset).Limit(pagination.Limit).Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// CreateItem creates a new item with the given name and description
// Returns the created item or an error if the operation fails
func (s *ItemServiceDefault) CreateItem(name string, description string) (*models.Item, error) {
	item := &models.Item{
		Name:        name,
		Description: description,
	}

	if err := s.db.Create(item).Error; err != nil {
		return nil, err
	}

	return item, nil
}

// GetItem retrieves a single item by its ID
// Returns the item if found, or an error if not found or operation fails
func (s *ItemServiceDefault) GetItem(id uint64) (*models.Item, error) {
	var item models.Item
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// UpdateItem updates an existing item with new values
// Returns an error if the item doesn't exist or the update fails
func (s *ItemServiceDefault) UpdateItem(id uint64, name string, description string) error {
	var item models.Item
	if err := s.db.First(&item, id).Error; err != nil {
		return err
	}

	item.Name = name
	item.Description = description

	return s.db.Save(&item).Error
}

// DeleteItem removes an item from the database
// Returns an error if the item doesn't exist or the deletion fails
func (s *ItemServiceDefault) DeleteItem(id uint64) error {
	return s.db.Delete(&models.Item{}, id).Error
}

// SearchItems performs a text search on item names and descriptions
// Returns matching items up to the specified limit, total count of matches, and any error
func (s *ItemServiceDefault) SearchItems(query string, limit int) ([]models.Item, int64, error) {
	var items []models.Item
	var total int64

	searchQuery := "%" + query + "%"

	if err := s.db.Model(&models.Item{}).Where(
		"name LIKE ? OR description LIKE ?",
		searchQuery, searchQuery,
	).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := s.db.Where(
		"name LIKE ? OR description LIKE ?",
		searchQuery, searchQuery,
	).Limit(limit).Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}
