package database

import (
	"fmt"

	"gorm.io/gorm"
)

// SourceRepository 数据源仓储
// Author: STRMSync Team
type SourceRepository struct{}

// Create 创建数据源
func (r *SourceRepository) Create(source *Source) error {
	return DB.Create(source).Error
}

// GetByID 根据ID获取数据源
func (r *SourceRepository) GetByID(id uint) (*Source, error) {
	var source Source
	err := DB.First(&source, id).Error
	if err != nil {
		return nil, err
	}
	return &source, nil
}

// GetAll 获取所有数据源
func (r *SourceRepository) GetAll() ([]Source, error) {
	var sources []Source
	err := DB.Find(&sources).Error
	return sources, err
}

// Update 更新数据源
func (r *SourceRepository) Update(source *Source) error {
	return DB.Save(source).Error
}

// Delete 删除数据源
func (r *SourceRepository) Delete(id uint) error {
	return DB.Delete(&Source{}, id).Error
}

// FileRepository 文件仓储
// Author: STRMSync Team
type FileRepository struct{}

// Create 创建文件记录
func (r *FileRepository) Create(file *File) error {
	return DB.Create(file).Error
}

// BatchCreate 批量创建文件记录
func (r *FileRepository) BatchCreate(files []File) error {
	return DB.CreateInBatches(files, 1000).Error
}

// GetByID 根据ID获取文件
func (r *FileRepository) GetByID(id uint) (*File, error) {
	var file File
	err := DB.Preload("Source").First(&file, id).Error
	if err != nil {
		return nil, err
	}
	return &file, nil
}

// GetBySourceID 获取数据源的所有文件
func (r *FileRepository) GetBySourceID(sourceID uint, limit, offset int) ([]File, error) {
	var files []File
	err := DB.Where("source_id = ?", sourceID).
		Limit(limit).
		Offset(offset).
		Find(&files).Error
	return files, err
}

// Update 更新文件记录
func (r *FileRepository) Update(file *File) error {
	return DB.Save(file).Error
}

// Delete 删除文件记录
func (r *FileRepository) Delete(id uint) error {
	return DB.Delete(&File{}, id).Error
}

// CountBySourceID 统计数据源的文件数量
func (r *FileRepository) CountBySourceID(sourceID uint) (int64, error) {
	var count int64
	err := DB.Model(&File{}).Where("source_id = ?", sourceID).Count(&count).Error
	return count, err
}

// TaskRepository 任务仓储
// Author: STRMSync Team
type TaskRepository struct{}

// Create 创建任务
func (r *TaskRepository) Create(task *Task) error {
	return DB.Create(task).Error
}

// GetByID 根据ID获取任务
func (r *TaskRepository) GetByID(id uint) (*Task, error) {
	var task Task
	err := DB.First(&task, id).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// GetAll 获取所有任务
func (r *TaskRepository) GetAll(limit, offset int) ([]Task, error) {
	var tasks []Task
	err := DB.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&tasks).Error
	return tasks, err
}

// Update 更新任务
func (r *TaskRepository) Update(task *Task) error {
	return DB.Save(task).Error
}

// UpdateStatus 更新任务状态
func (r *TaskRepository) UpdateStatus(id uint, status string, message string) error {
	return DB.Model(&Task{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":  status,
			"message": message,
		}).Error
}

// SettingRepository 设置仓储
// Author: STRMSync Team
type SettingRepository struct{}

// Get 获取设置
func (r *SettingRepository) Get(key string) (*Setting, error) {
	var setting Setting
	err := DB.Where("key = ?", key).First(&setting).Error
	if err != nil {
		return nil, err
	}
	return &setting, nil
}

// Set 设置配置
func (r *SettingRepository) Set(key, value, category string) error {
	setting := Setting{
		Key:      key,
		Value:    value,
		Category: category,
	}

	// 使用 Upsert 操作
	return DB.Where(Setting{Key: key}).Assign(setting).FirstOrCreate(&setting).Error
}

// GetByCategory 获取分类下的所有设置
func (r *SettingRepository) GetByCategory(category string) ([]Setting, error) {
	var settings []Setting
	err := DB.Where("category = ?", category).Find(&settings).Error
	return settings, err
}

// Delete 删除设置
func (r *SettingRepository) Delete(key string) error {
	return DB.Where("key = ?", key).Delete(&Setting{}).Error
}

// NewSourceRepository 创建数据源仓储实例
func NewSourceRepository() *SourceRepository {
	return &SourceRepository{}
}

// NewFileRepository 创建文件仓储实例
func NewFileRepository() *FileRepository {
	return &FileRepository{}
}

// NewTaskRepository 创建任务仓储实例
func NewTaskRepository() *TaskRepository {
	return &TaskRepository{}
}

// NewSettingRepository 创建设置仓储实例
func NewSettingRepository() *SettingRepository {
	return &SettingRepository{}
}

// Transaction 执行事务
// Author: STRMSync Team
func Transaction(fn func(*gorm.DB) error) error {
	return DB.Transaction(fn)
}

// HealthCheck 数据库健康检查
// Author: STRMSync Team
func HealthCheck() error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	return sqlDB.Ping()
}
