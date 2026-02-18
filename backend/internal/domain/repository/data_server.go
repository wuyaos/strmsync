package repository

import (
	"context"

	"github.com/strmsync/strmsync/internal/domain/model"
)

// DataServerRepository DataServer仓储接口
type DataServerRepository interface {
	GetByID(ctx context.Context, id uint) (model.DataServer, error)
}
