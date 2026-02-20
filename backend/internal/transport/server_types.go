// Package http 提供HTTP API处理器
package http

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ServerFieldType 字段类型枚举
type ServerFieldType string

const (
	FieldTypeText     ServerFieldType = "text"
	FieldTypeNumber   ServerFieldType = "number"
	FieldTypePassword ServerFieldType = "password"
	FieldTypeSelect   ServerFieldType = "select"
	FieldTypeRadio    ServerFieldType = "radio"
	FieldTypePath     ServerFieldType = "path"
	FieldTypeHidden   ServerFieldType = "hidden"
)

// FieldOption 下拉选项定义
type FieldOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// ServerFieldDef 字段定义结构
// 描述表单字段的所有展示和验证属性
type ServerFieldDef struct {
	Name        string            `json:"name"`                  // 字段名称
	Type        ServerFieldType   `json:"type"`                  // 字段类型
	Label       string            `json:"label,omitempty"`       // 显示标签
	Placeholder string            `json:"placeholder,omitempty"` // 占位符文本
	Help        string            `json:"help,omitempty"`        // 帮助提示
	Required    bool              `json:"required,omitempty"`    // 是否必填
	Default     interface{}       `json:"default,omitempty"`     // 默认值
	Options     []FieldOption     `json:"options,omitempty"`     // 下拉选项（仅 select 类型）
	Min         *int              `json:"min,omitempty"`         // 最小值（仅 number 类型）
	Max         *int              `json:"max,omitempty"`         // 最大值（仅 number 类型）
	VisibleIf   map[string]string `json:"visible_if,omitempty"`  // 条件显示规则
	ColSpan     int               `json:"col_span,omitempty"`    // 列宽（24栅格系统）
}

// ServerSectionDef 字段分组定义
// 用于将字段按逻辑分组，支持不同布局
type ServerSectionDef struct {
	ID      string           `json:"id"`                // 分组唯一标识
	Label   string           `json:"label"`             // 分组显示标签
	Layout  string           `json:"layout,omitempty"`  // 布局方式：row（横向）或默认（纵向）
	Columns int              `json:"columns,omitempty"` // 列数（横向布局时使用）
	Fields  []ServerFieldDef `json:"fields"`            // 字段列表
}

// ServerTypeDef 服务器类型完整定义
// 包含类型元信息、字段分组、存储映射和版本号
type ServerTypeDef struct {
	Type         string             `json:"type"`                  // 类型标识（如 local/clouddrive2）
	Label        string             `json:"label"`                 // 类型显示名称
	Category     string             `json:"category"`              // 类型分类（data/media）
	Description  string             `json:"description,omitempty"` // 类型描述
	Sections     []ServerSectionDef `json:"sections"`              // 字段分组
	Storage      map[string]string  `json:"storage"`               // 字段到存储位置的映射（root/options/api_key）
	RulesVersion int                `json:"rules_version"`         // 规则版本号（用于缓存失效）
}

// ServerTypeHandler 服务器类型处理器
type ServerTypeHandler struct{}

// NewServerTypeHandler 创建服务器类型处理器实例
func NewServerTypeHandler() *ServerTypeHandler {
	return &ServerTypeHandler{}
}

// ListServerTypes 获取服务器类型列表
// GET /api/servers/types?category=data
func (h *ServerTypeHandler) ListServerTypes(c *gin.Context) {
	category := strings.TrimSpace(c.Query("category"))
	types := getServerTypeDefinitions()

	// 根据分类过滤
	if category != "" {
		filtered := make([]ServerTypeDef, 0, len(types))
		for _, def := range types {
			if def.Category == category {
				filtered = append(filtered, def)
			}
		}
		types = filtered
	}

	c.JSON(http.StatusOK, gin.H{"types": types})
}

// GetServerType 获取单个服务器类型定义
// GET /api/servers/types/:type
func (h *ServerTypeHandler) GetServerType(c *gin.Context) {
	serverType := strings.TrimSpace(c.Param("type"))
	def, ok := findServerTypeDefinition(serverType)
	if !ok {
		respondError(c, http.StatusNotFound, "not_found", "服务器类型不存在", nil)
		return
	}

	c.JSON(http.StatusOK, gin.H{"type": def})
}

// findServerTypeDefinition 查找指定类型的定义
func findServerTypeDefinition(serverType string) (ServerTypeDef, bool) {
	for _, def := range getServerTypeDefinitions() {
		if def.Type == serverType {
			return def, true
		}
	}
	return ServerTypeDef{}, false
}

// getServerTypeDefinitions 获取所有服务器类型定义
func getServerTypeDefinitions() []ServerTypeDef {
	return []ServerTypeDef{
		localServerTypeDef(),
		cloudDrive2ServerTypeDef(),
		openListServerTypeDef(),
	}
}

// localServerTypeDef 本地文件系统服务器类型定义
func localServerTypeDef() ServerTypeDef {
	return ServerTypeDef{
		Type:         "local",
		Label:        "Local",
		Category:     "data",
		Description:  "本地文件系统",
		RulesVersion: 1,
		Sections: []ServerSectionDef{
			{
				ID:    "paths",
				Label: "路径配置",
				Fields: []ServerFieldDef{
					{
						Name:        "access_path",
						Type:        FieldTypePath,
						Label:       "访问目录",
						Placeholder: "例如：/mnt/data",
						Help:        "本软件可访问数据的本地目录路径",
						Required:    true,
					},
					{
						Name:        "mount_path",
						Type:        FieldTypePath,
						Label:       "挂载目录",
						Placeholder: "可选，一般与访问目录一致",
						Help:        "云盘挂载的本地目录路径",
						Required:    false,
					},
				},
			},
		},
		Storage: map[string]string{
			"access_path": "options",
			"mount_path":  "options",
		},
	}
}

// cloudDrive2ServerTypeDef CloudDrive2 云盘挂载服务器类型定义
func cloudDrive2ServerTypeDef() ServerTypeDef {
	minPort := 1
	maxPort := 65535

	return ServerTypeDef{
		Type:         "clouddrive2",
		Label:        "CloudDrive2",
		Category:     "data",
		Description:  "云盘挂载服务（gRPC）",
		RulesVersion: 2,
		Sections: []ServerSectionDef{
			{
				ID:     "auth",
				Label:  "认证信息",
				Layout: "row",
				Fields: []ServerFieldDef{
					{
						Name:        "host",
						Type:        FieldTypeText,
						Label:       "主机地址",
						Placeholder: "192.168.1.100",
						Required:    true,
						ColSpan:     14,
					},
					{
						Name:     "port",
						Type:     FieldTypeNumber,
						Label:    "端口号",
						Required: true,
						Default:  19798,
						Min:      &minPort,
						Max:      &maxPort,
						ColSpan:  10,
					},
					{
						Name:        "api_token",
						Type:        FieldTypePassword,
						Label:       "API 令牌",
						Placeholder: "请输入 gRPC Token",
						Help:        "CloudDrive2 的 gRPC 认证令牌",
						Required:    true,
						ColSpan:     24,
					},
				},
			},
			{
				ID:    "paths",
				Label: "路径配置",
				Fields: []ServerFieldDef{
					{
						Name:        "access_path",
						Type:        FieldTypePath,
						Label:       "访问目录",
						Placeholder: "/CloudNAS",
						Help:        "本软件可访问的目录路径（用于读取云盘内容）",
						Required:    true,
					},
					{
						Name:        "mount_path",
						Type:        FieldTypePath,
						Label:       "挂载目录",
						Placeholder: "/mnt/clouddrive",
						Help:        "云盘挂载到本地的目录路径（可选，默认使用访问目录）",
						Required:    false,
					},
				},
			},
		},
		Storage: map[string]string{
			"host":        "root",
			"port":        "root",
			"api_token":   "api_key",
			"access_path": "options",
			"mount_path":  "options",
		},
	}
}

// openListServerTypeDef OpenList API 服务器类型定义
func openListServerTypeDef() ServerTypeDef {
	minPort := 1
	maxPort := 65535

	return ServerTypeDef{
		Type:         "openlist",
		Label:        "OpenList",
		Category:     "data",
		Description:  "OpenList API 服务",
		RulesVersion: 2,
		Sections: []ServerSectionDef{
			{
				ID:     "auth",
				Label:  "认证信息",
				Layout: "row",
				Fields: []ServerFieldDef{
					{
						Name:        "host",
						Type:        FieldTypeText,
						Label:       "主机地址",
						Placeholder: "192.168.1.200",
						Required:    true,
						ColSpan:     14,
					},
					{
						Name:     "port",
						Type:     FieldTypeNumber,
						Label:    "端口号",
						Required: true,
						Default:  5244,
						Min:      &minPort,
						Max:      &maxPort,
						ColSpan:  10,
					},
					{
						Name:        "username",
						Type:        FieldTypeText,
						Label:       "用户名",
						Placeholder: "admin",
						Required:    true,
						Help:        "使用用户名密码登录获取 token",
						ColSpan:     12,
					},
					{
						Name:        "password",
						Type:        FieldTypePassword,
						Label:       "密码",
						Placeholder: "请输入密码",
						Required:    true,
						ColSpan:     12,
					},
				},
			},
			{
				ID:    "paths",
				Label: "路径配置",
				Fields: []ServerFieldDef{
					{
						Name:        "access_path",
						Type:        FieldTypePath,
						Label:       "访问目录",
						Placeholder: "/",
						Help:        "本地方式可填写访问目录；挂载目录为空时默认使用访问目录",
						Required:    false,
					},
					{
						Name:        "mount_path",
						Type:        FieldTypePath,
						Label:       "挂载目录",
						Placeholder: "/mnt/openlist",
						Help:        "挂载目录可选，默认使用访问目录",
						Required:    false,
					},
				},
			},
		},
		Storage: map[string]string{
			"host":        "root",
			"port":        "root",
			"username":    "options",
			"password":    "options",
			"access_path": "options",
			"mount_path":  "options",
		},
	}
}
