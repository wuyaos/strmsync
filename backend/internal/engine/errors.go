// Package syncengine provides shared error values for the driver layer.
//
// These sentinel errors allow callers to check for specific error conditions
// using errors.Is(). Drivers should wrap these errors with additional context.
package syncengine

import "errors"

var (
	// ErrNotSupported indicates the operation is not supported by a driver.
	//
	// This is returned when a driver's Capabilities indicate a feature is unavailable.
	// For example, calling Watch on a driver with Capabilities().Watch == false.
	//
	// Usage:
	//   if errors.Is(err, syncengine.ErrNotSupported) {
	//       // Handle unsupported operation
	//   }
	ErrNotSupported = errors.New("syncengine: operation not supported")

	// ErrInvalidInput indicates a caller provided invalid input.
	//
	// This is returned when input validation fails, such as:
	// - Empty required parameters
	// - Invalid DriverType
	// - Malformed paths
	// - Out-of-range values
	//
	// Drivers should wrap this error with specific details:
	//   return fmt.Errorf("invalid path %q: %w", path, ErrInvalidInput)
	ErrInvalidInput = errors.New("syncengine: invalid input")

	// ErrInvalidStrm indicates STRM content is malformed or incomplete.
	//
	// This is returned by CompareStrm when the actual STRM content
	// cannot be parsed or validated, such as:
	// - Invalid URL format
	// - Missing required components (BaseURL, Path)
	// - Corrupted or truncated content
	//
	// Unlike comparison mismatches (which return CompareResult with NeedUpdate=true),
	// this error indicates the comparison cannot proceed at all.
	ErrInvalidStrm = errors.New("syncengine: invalid strm content")
)
