// Package database provides a basic repository abstraction for GORM.
package database

import (
	"errors"

	"gorm.io/gorm"
)

// Repository defines minimal access to the underlying DB.
// It can be embedded by domain-specific repositories.
type Repository interface {
	DB() (*gorm.DB, error)
}

// BaseRepository provides common CRUD helpers.
// It can be reused for any model type.
type BaseRepository struct {
	db *gorm.DB
}

// NewBaseRepository creates a repository using the provided DB.
// If db is nil, it will fall back to the global instance.
func NewBaseRepository(db *gorm.DB) *BaseRepository {
	return &BaseRepository{db: db}
}

// DB returns the effective database handle.
func (r *BaseRepository) DB() (*gorm.DB, error) {
	if r.db != nil {
		return r.db, nil
	}
	return GetDB()
}

// WithTx returns a new repository bound to the given transaction.
func (r *BaseRepository) WithTx(tx *gorm.DB) (*BaseRepository, error) {
	if tx == nil {
		return nil, errors.New("transaction is nil")
	}
	return &BaseRepository{db: tx}, nil
}

// Create inserts a new record.
func (r *BaseRepository) Create(model any) error {
	db, err := r.DB()
	if err != nil {
		return err
	}
	return db.Create(model).Error
}

// Save updates all fields for the given model.
func (r *BaseRepository) Save(model any) error {
	db, err := r.DB()
	if err != nil {
		return err
	}
	return db.Save(model).Error
}

// First finds the first matching record into dest.
func (r *BaseRepository) First(dest any, conds ...any) error {
	db, err := r.DB()
	if err != nil {
		return err
	}
	return db.First(dest, conds...).Error
}

// Find queries records into dest.
func (r *BaseRepository) Find(dest any, conds ...any) error {
	db, err := r.DB()
	if err != nil {
		return err
	}
	return db.Find(dest, conds...).Error
}

// Delete removes records by model or condition.
func (r *BaseRepository) Delete(model any, conds ...any) error {
	db, err := r.DB()
	if err != nil {
		return err
	}
	return db.Delete(model, conds...).Error
}

// Update updates a single column for the given model.
func (r *BaseRepository) Update(model any, column string, value any) error {
	db, err := r.DB()
	if err != nil {
		return err
	}
	return db.Model(model).Update(column, value).Error
}

// Count counts records for a model into count.
func (r *BaseRepository) Count(model any, count *int64) error {
	if count == nil {
		return errors.New("count is nil")
	}
	db, err := r.DB()
	if err != nil {
		return err
	}
	return db.Model(model).Count(count).Error
}
