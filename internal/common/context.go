package common

// Context key types for avoiding collisions
type contextKey string

const (
	// VerboseKey is the context key for verbose flag
	VerboseKey contextKey = "verbose"
	// DebugKey is the context key for debug flag
	DebugKey contextKey = "debug"
)