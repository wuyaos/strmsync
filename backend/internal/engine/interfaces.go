// Package syncengine defines interfaces for the unified driver layer.
//
// The Driver interface provides a clean abstraction over different data sources,
// allowing the sync engine to work uniformly with CloudDrive2, OpenList, and
// local filesystems without knowing implementation details.
package syncengine

import "context"

// Driver is the unified interface that all data source drivers must implement.
//
// This interface defines the contract between the sync engine and data source providers.
// Implementations must:
// - Handle context cancellation gracefully
// - Wrap errors with context using fmt.Errorf("operation: %w", err)
// - Return ErrNotSupported for unsupported operations
// - Be safe for concurrent use by multiple goroutines
//
// Design Philosophy:
// - Each method focuses on a single responsibility
// - Methods provide structured data rather than strings where possible
// - Capabilities are declared upfront to allow engine adaptation
// - Errors are typed and wrapped for clear diagnostics
type Driver interface {
	// Type returns the driver type identifier.
	//
	// This is used for logging, metrics, and capability routing.
	Type() DriverType

	// Capabilities returns the features supported by this driver.
	//
	// The sync engine uses this to:
	// - Enable/disable Watch mode
	// - Choose between HTTP and mount-based STRMs
	// - Determine if PickCode/Sign validation is needed
	Capabilities() DriverCapability

	// List returns remote entries under the given path.
	//
	// Parameters:
	// - ctx: Context for cancellation and timeout
	// - path: Remote path to list (empty or "/" for root)
	// - opt: List options (recursive, max depth)
	//
	// Returns:
	// - []RemoteEntry: List of files and directories
	// - error: Any error encountered
	//
	// Behavior:
	// - Respects opt.Recursive and opt.MaxDepth
	// - Returns empty slice (not nil) when path is empty
	// - Handles pagination internally
	// - Normalizes paths to Unix format
	//
	// Error Handling:
	// - Returns context.Canceled if ctx is cancelled
	// - Returns wrapped error with path context
	List(ctx context.Context, path string, opt ListOptions) ([]RemoteEntry, error)

	// Watch subscribes to file change events for the given path.
	//
	// Parameters:
	// - ctx: Context for cancellation (closes channel when cancelled)
	// - path: Remote path to watch
	// - opt: Watch options (recursive)
	//
	// Returns:
	// - <-chan DriverEvent: Event channel (closed when watch stops)
	// - error: ErrNotSupported if Watch is not available, or other errors
	//
	// Behavior:
	// - Returns ErrNotSupported if Capabilities().Watch is false
	// - Channel is closed when ctx is cancelled or watch fails
	// - Events are sent asynchronously
	// - May buffer events internally to prevent blocking
	//
	// Important:
	// - Caller must consume events to prevent goroutine leaks
	// - Watch should run in a separate goroutine
	Watch(ctx context.Context, path string, opt WatchOptions) (<-chan DriverEvent, error)

	// Stat fetches metadata for a single remote path.
	//
	// Parameters:
	// - ctx: Context for cancellation and timeout
	// - path: Remote file or directory path
	//
	// Returns:
	// - RemoteEntry: File metadata
	// - error: Error if path doesn't exist or access fails
	//
	// Behavior:
	// - More efficient than List for single file queries
	// - Returns error (not zero-value) if path doesn't exist
	// - May use cached data if driver supports caching
	//
	// Fallback:
	// - Drivers without native Stat can implement using List
	Stat(ctx context.Context, path string) (RemoteEntry, error)

	// BuildStrmInfo constructs STRM content and metadata for a remote path.
	//
	// Parameters:
	// - ctx: Context for cancellation and timeout
	// - req: Build request with path and optional metadata
	//
	// Returns:
	// - StrmInfo: Structured STRM information
	// - error: Any error during construction
	//
	// Behavior:
	// - Generates appropriate URL based on driver type
	// - Includes PickCode if Capabilities().PickCode is true
	// - Includes Sign/ExpiresAt if Capabilities().SignURL is true
	// - Can use req.RemoteMeta to avoid redundant metadata fetch
	//
	// URL Format Examples:
	// - CloudDrive2: http://host:port/path?sign=xxx&expires=123
	// - OpenList: http://host:port/d/path?sign=xxx
	// - Local: file:///absolute/path or /absolute/path
	BuildStrmInfo(ctx context.Context, req BuildStrmRequest) (StrmInfo, error)

	// CompareStrm compares existing STRM content to expected content.
	//
	// Parameters:
	// - ctx: Context (reserved for future use)
	// - input: Expected info and actual raw content
	//
	// Returns:
	// - CompareResult: Comparison outcome with reason
	// - error: Only for invalid input (not comparison mismatch)
	//
	// Behavior:
	// - Returns Equal=true, NeedUpdate=false if identical
	// - Returns Equal=false, NeedUpdate=true if mismatch
	// - Checks BaseURL, Path, PickCode, Sign, and expiration
	// - Ignores whitespace and trailing slashes
	// - Returns NeedUpdate=true if parsing fails
	//
	// Comparison Rules:
	// 1. Empty actualRaw → NeedUpdate
	// 2. Parse failure → NeedUpdate
	// 3. BaseURL mismatch → NeedUpdate
	// 4. Path mismatch (after normalization) → NeedUpdate
	// 5. PickCode mismatch (if PickCode capability) → NeedUpdate
	// 6. Sign missing or expired (if SignURL capability) → NeedUpdate
	// 7. All checks pass → Equal
	CompareStrm(ctx context.Context, input CompareInput) (CompareResult, error)

	// TestConnection validates connectivity to the data source.
	//
	// Parameters:
	// - ctx: Context for timeout
	//
	// Returns:
	// - error: nil if connection is healthy, error otherwise
	//
	// Behavior:
	// - Makes a lightweight API call to verify connectivity
	// - Should complete quickly (use ctx timeout)
	// - May cache recent test results to avoid rate limits
	//
	// Use Cases:
	// - Server configuration validation
	// - Health checks
	// - Pre-sync connectivity verification
	TestConnection(ctx context.Context) error
}
