// Package core 提供 DataServer 相关的 GORM Repository 实现
package core

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// GormDataServerRepository 是基于 GORM 的 DataServer 数据访问实现
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

// GetByID 获取指定 DataServer
//
// 参数：
//   - ctx: 上下文（为 nil 时自动使用 Background）
//   - id: DataServer ID
//
// 返回：
//   - DataServer: DataServer 对象
//   - error: 查询失败或不存在时返回 gorm.ErrRecordNotFound
func (r *GormDataServerRepository) GetByID(ctx context.Context, id uint) (DataServer, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var server DataServer
	if err := r.db.WithContext(ctx).First(&server, id).Error; err != nil {
		return DataServer{}, err
	}
	return server, nil
}
