// Package mediaserver provides media server adapter.
//
// This file exists only for backward compatibility with the old structure.
// The actual media server client is now in internal/pkg/sdk/mediaserver.
package mediaserver

import (
	sdk "github.com/strmsync/strmsync/internal/pkg/sdk/mediaserver"
)

// Re-export SDK types for backward compatibility
type (
	Client = sdk.Client
	Config = sdk.Config
	Type   = sdk.Type
	Option = sdk.Option
)

// Re-export SDK constants
const (
	TypeEmby     = sdk.TypeEmby
	TypeJellyfin = sdk.TypeJellyfin
)

// Re-export SDK functions
var (
	NewClient      = sdk.NewClient
	WithHTTPClient = sdk.WithHTTPClient
	WithLogger     = sdk.WithLogger
)
