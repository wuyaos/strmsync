package model

import (
	"strings"
	"testing"
)

// ---------------------
// UID生成测试
// ---------------------

func TestGenerateDataServerUID(t *testing.T) {
	tests := []struct {
		name        string
		serverType  string
		host        string
		port        int
		options     string
		apiKey      string
		wantErr     bool
		description string
	}{
		{
			name:        "basic clouddrive2 server",
			serverType:  "clouddrive2",
			host:        "192.168.1.100",
			port:        19798,
			options:     "{}",
			apiKey:      "test-key",
			wantErr:     false,
			description: "基本的CloudDrive2服务器配置",
		},
		{
			name:        "openlist server with options",
			serverType:  "openlist",
			host:        "localhost",
			port:        8080,
			options:     `{"timeout": 30, "retry": 3}`,
			apiKey:      "",
			wantErr:     false,
			description: "OpenList服务器带扩展选项",
		},
		{
			name:        "empty options",
			serverType:  "clouddrive2",
			host:        "server.local",
			port:        443,
			options:     "",
			apiKey:      "key123",
			wantErr:     false,
			description: "空options应被规范化为{}",
		},
		{
			name:        "options with false and zero",
			serverType:  "openlist",
			host:        "api.example.com",
			port:        8443,
			options:     `{"enabled": false, "retry": 0, "timeout": 30}`,
			apiKey:      "secret",
			wantErr:     false,
			description: "false和0应保留（不被过滤）",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uid, err := GenerateDataServerUID(tt.serverType, tt.host, tt.port, tt.options, tt.apiKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateDataServerUID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// UID应该是64字符的hex字符串（SHA-256）
				if len(uid) != 64 {
					t.Errorf("UID length = %d, want 64", len(uid))
				}
				// UID应该只包含hex字符
				if !isHexString(uid) {
					t.Errorf("UID contains non-hex characters: %s", uid)
				}
			}
		})
	}
}

func TestGenerateMediaServerUID(t *testing.T) {
	tests := []struct {
		name       string
		serverType string
		host       string
		port       int
		options    string
		apiKey     string
		wantErr    bool
	}{
		{
			name:       "emby server",
			serverType: "emby",
			host:       "192.168.1.200",
			port:       8096,
			options:    "{}",
			apiKey:     "emby-key",
			wantErr:    false,
		},
		{
			name:       "jellyfin server",
			serverType: "jellyfin",
			host:       "jellyfin.local",
			port:       8096,
			options:    "{}",
			apiKey:     "jf-key",
			wantErr:    false,
		},
		{
			name:       "plex server with complex options",
			serverType: "plex",
			host:       "plex.example.com",
			port:       32400,
			options:    `{"library": "Movies", "quality": "original"}`,
			apiKey:     "plex-token",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uid, err := GenerateMediaServerUID(tt.serverType, tt.host, tt.port, tt.options, tt.apiKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateMediaServerUID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(uid) != 64 {
					t.Errorf("UID length = %d, want 64", len(uid))
				}
				if !isHexString(uid) {
					t.Errorf("UID contains non-hex characters: %s", uid)
				}
			}
		})
	}
}

// ---------------------
// UID稳定性测试
// ---------------------

func TestUID_Stability(t *testing.T) {
	// 相同参数应生成相同UID
	uid1, err1 := GenerateDataServerUID("clouddrive2", "192.168.1.100", 19798, "{}", "key")
	uid2, err2 := GenerateDataServerUID("clouddrive2", "192.168.1.100", 19798, "{}", "key")

	if err1 != nil || err2 != nil {
		t.Fatalf("GenerateDataServerUID() error: %v, %v", err1, err2)
	}
	if uid1 != uid2 {
		t.Errorf("相同参数生成了不同的UID:\nUID1: %s\nUID2: %s", uid1, uid2)
	}
}

func TestUID_HostCaseInsensitive(t *testing.T) {
	// Host大小写不应影响UID
	uid1, _ := GenerateDataServerUID("clouddrive2", "Server.Local", 19798, "{}", "key")
	uid2, _ := GenerateDataServerUID("clouddrive2", "server.local", 19798, "{}", "key")
	uid3, _ := GenerateDataServerUID("clouddrive2", "SERVER.LOCAL", 19798, "{}", "key")

	if uid1 != uid2 || uid2 != uid3 {
		t.Errorf("Host大小写导致不同的UID:\nUID1: %s\nUID2: %s\nUID3: %s", uid1, uid2, uid3)
	}
}

func TestUID_OptionOrderIndependent(t *testing.T) {
	// JSON选项的key顺序不应影响UID
	uid1, _ := GenerateDataServerUID("clouddrive2", "host", 80, `{"a":1,"b":2,"c":3}`, "key")
	uid2, _ := GenerateDataServerUID("clouddrive2", "host", 80, `{"c":3,"a":1,"b":2}`, "key")
	uid3, _ := GenerateDataServerUID("clouddrive2", "host", 80, `{"b":2,"c":3,"a":1}`, "key")

	if uid1 != uid2 || uid2 != uid3 {
		t.Errorf("JSON key顺序导致不同的UID:\nUID1: %s\nUID2: %s\nUID3: %s", uid1, uid2, uid3)
	}
}

func TestUID_DifferentConfigurations(t *testing.T) {
	// 不同配置应生成不同UID
	tests := []struct {
		name       string
		serverType string
		host       string
		port       int
		options    string
		apiKey     string
	}{
		{"config1", "clouddrive2", "host1", 19798, "{}", "key1"},
		{"config2", "clouddrive2", "host2", 19798, "{}", "key1"}, // 不同host
		{"config3", "clouddrive2", "host1", 19799, "{}", "key1"}, // 不同port
		{"config4", "clouddrive2", "host1", 19798, `{"a":1}`, "key1"}, // 不同options
		{"config5", "clouddrive2", "host1", 19798, "{}", "key2"}, // 不同apikey
		{"config6", "openlist", "host1", 19798, "{}", "key1"}, // 不同type
	}

	uids := make(map[string]string)
	for _, tt := range tests {
		uid, err := GenerateDataServerUID(tt.serverType, tt.host, tt.port, tt.options, tt.apiKey)
		if err != nil {
			t.Fatalf("%s: GenerateDataServerUID() error = %v", tt.name, err)
		}
		uids[tt.name] = uid
	}

	// 检查所有UID是否唯一
	seen := make(map[string]string)
	for name, uid := range uids {
		if existingName, exists := seen[uid]; exists {
			t.Errorf("配置 %s 和 %s 生成了相同的UID: %s", name, existingName, uid)
		}
		seen[uid] = name
	}
}

// ---------------------
// BuildXXXUIDForUpdate测试
// ---------------------

func TestBuildDataServerUIDForUpdate(t *testing.T) {
	// 生成初始UID
	originalUID, _ := GenerateDataServerUID("clouddrive2", "host", 19798, "{}", "key")

	tests := []struct {
		name         string
		currentUID   string
		serverType   string
		host         string
		port         int
		options      string
		apiKey       string
		wantChanged  bool
		description  string
	}{
		{
			name:         "no change",
			currentUID:   originalUID,
			serverType:   "clouddrive2",
			host:         "host",
			port:         19798,
			options:      "{}",
			apiKey:       "key",
			wantChanged:  false,
			description:  "相同配置不应改变UID",
		},
		{
			name:         "host changed",
			currentUID:   originalUID,
			serverType:   "clouddrive2",
			host:         "newhost",
			port:         19798,
			options:      "{}",
			apiKey:       "key",
			wantChanged:  true,
			description:  "Host变化应导致UID改变",
		},
		{
			name:         "port changed",
			currentUID:   originalUID,
			serverType:   "clouddrive2",
			host:         "host",
			port:         19799,
			options:      "{}",
			apiKey:       "key",
			wantChanged:  true,
			description:  "Port变化应导致UID改变",
		},
		{
			name:         "options changed",
			currentUID:   originalUID,
			serverType:   "clouddrive2",
			host:         "host",
			port:         19798,
			options:      `{"timeout":30}`,
			apiKey:       "key",
			wantChanged:  true,
			description:  "Options变化应导致UID改变",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newUID, changed, err := BuildDataServerUIDForUpdate(tt.currentUID, tt.serverType, tt.host, tt.port, tt.options, tt.apiKey)
			if err != nil {
				t.Fatalf("BuildDataServerUIDForUpdate() error = %v", err)
			}
			if changed != tt.wantChanged {
				t.Errorf("changed = %v, want %v (description: %s)", changed, tt.wantChanged, tt.description)
			}
			if !changed && newUID != tt.currentUID {
				t.Errorf("UID changed but changed flag is false: old=%s, new=%s", tt.currentUID, newUID)
			}
			if changed && newUID == tt.currentUID {
				t.Errorf("changed flag is true but UID not changed: %s", newUID)
			}
		})
	}
}

// ---------------------
// JSON规范化测试
// ---------------------

func TestNormalizeJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "{}",
			wantErr:  false,
		},
		{
			name:     "empty object",
			input:    "{}",
			expected: "{}",
			wantErr:  false,
		},
		{
			name:     "preserve false and zero",
			input:    `{"enabled": false, "retry": 0}`,
			expected: `{"enabled":false,"retry":0}`,
			wantErr:  false,
		},
		{
			name:     "filter empty string",
			input:    `{"name": "", "value": "test"}`,
			expected: `{"value":"test"}`,
			wantErr:  false,
		},
		{
			name:     "filter empty array and object",
			input:    `{"arr": [], "obj": {}, "value": 1}`,
			expected: `{"value":1}`,
			wantErr:  false,
		},
		{
			name:     "recursive sorting",
			input:    `{"z": {"b": 2, "a": 1}, "a": 3}`,
			expected: `{"a":3,"z":{"a":1,"b":2}}`,
			wantErr:  false,
		},
		{
			name:     "invalid json returns original",
			input:    `not a json`,
			expected: `not a json`,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := normalizeJSON(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("normalizeJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.expected {
				t.Errorf("normalizeJSON() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestIsEmptyValue(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected bool
	}{
		{"nil", nil, true},
		{"empty string", "", true},
		{"empty map", map[string]any{}, true},
		{"empty slice", []any{}, true},
		{"false", false, false}, // false不是空值
		{"zero int", 0, false},   // 0不是空值
		{"zero float", 0.0, false},
		{"non-empty string", "test", false},
		{"non-empty map", map[string]any{"a": 1}, false},
		{"non-empty slice", []any{1, 2}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isEmptyValue(tt.value)
			if result != tt.expected {
				t.Errorf("isEmptyValue(%v) = %v, want %v", tt.value, result, tt.expected)
			}
		})
	}
}

// ---------------------
// 辅助函数
// ---------------------

func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func TestSortedKeys(t *testing.T) {
	m := map[string]any{
		"z": 1,
		"a": 2,
		"m": 3,
	}
	keys := sortedKeys(m)
	expected := []string{"a", "m", "z"}

	if len(keys) != len(expected) {
		t.Fatalf("sortedKeys() length = %d, want %d", len(keys), len(expected))
	}

	for i, key := range keys {
		if key != expected[i] {
			t.Errorf("sortedKeys()[%d] = %s, want %s", i, key, expected[i])
		}
	}
}

// ---------------------
// BeforeCreate Hook测试
// ---------------------

func TestDataServer_BeforeCreate(t *testing.T) {
	server := &DataServer{
		Type:    "clouddrive2",
		Host:    "192.168.1.100",
		Port:    19798,
		Options: "{}",
		APIKey:  "test-key",
	}

	// BeforeCreate应该生成UID
	err := server.BeforeCreate(nil)
	if err != nil {
		t.Fatalf("BeforeCreate() error = %v", err)
	}

	if server.UID == "" {
		t.Error("BeforeCreate() did not generate UID")
	}
	if len(server.UID) != 64 {
		t.Errorf("UID length = %d, want 64", len(server.UID))
	}
}

func TestDataServer_BeforeCreate_PreserveExistingUID(t *testing.T) {
	existingUID := strings.Repeat("a", 64)
	server := &DataServer{
		UID:     existingUID,
		Type:    "clouddrive2",
		Host:    "192.168.1.100",
		Port:    19798,
		Options: "{}",
		APIKey:  "test-key",
	}

	// BeforeCreate不应覆盖已有的UID
	err := server.BeforeCreate(nil)
	if err != nil {
		t.Fatalf("BeforeCreate() error = %v", err)
	}

	if server.UID != existingUID {
		t.Errorf("BeforeCreate() changed existing UID from %s to %s", existingUID, server.UID)
	}
}

func TestMediaServer_BeforeCreate(t *testing.T) {
	server := &MediaServer{
		Type:    "emby",
		Host:    "192.168.1.200",
		Port:    8096,
		Options: "{}",
		APIKey:  "emby-key",
	}

	// BeforeCreate应该生成UID
	err := server.BeforeCreate(nil)
	if err != nil {
		t.Fatalf("BeforeCreate() error = %v", err)
	}

	if server.UID == "" {
		t.Error("BeforeCreate() did not generate UID")
	}
	if len(server.UID) != 64 {
		t.Errorf("UID length = %d, want 64", len(server.UID))
	}
}
