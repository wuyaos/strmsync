// Package repository 提供 DataServer 相关的 GORM Repository 实现
package repository

import (
	"context"
	"fmt"

	"github.com/strmsync/strmsync/internal/domain/model"
	"gorm.io/gorm"
)

// GormDataServerRepository 是基于 GORM 的 model.DataServer 数据访问实现
type GormDataServerRepository struct {
	db *gorm.DB
}

// NewGormDataServerRepository 创建 GormDataServerRepository 实例
//
// 参数：
//   - db: GORM 数据库连接（不能为 nil）
//
// 返回：
//   - *GormDataServerRepository: Repository 实例
//   - error: db 为 nil 时返回错误
func NewGormDataServerRepository(db *gorm.DB) (*GormDataServerRepository, error) {
	if db == nil {
		return nil, fmt.Errorf("core: gorm db is nil")
	}
	return &GormDataServerRepository{db: db}, nil
}

// GetByID 获取指定 model.DataServer
//
// 参数：
//   - ctx: 上下文（为 nil 时自动使用 Background）
//   - id: model.DataServer ID
//
// 返回：
//   - model.DataServer: model.DataServer 对象
//   - error: 查询失败或不存在时返回 gorm.ErrRecordNotFound
func (r *GormDataServerRepository) GetByID(ctx context.Context, id uint) (model.DataServer, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var server model.DataServer
	if err := r.db.WithContext(ctx).First(&server, id).Error; err != nil {
		return model.DataServer{}, err
	}
	return server, nil
}
