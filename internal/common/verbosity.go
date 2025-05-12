package common

import (
	"fmt"
	"io"
	"os"
)

// VerbosityLevel represents the level of verbosity for output
type VerbosityLevel int

const (
	// VerbosityQuiet produces minimal output, only showing critical information and errors
	VerbosityQuiet VerbosityLevel = iota
	// VerbosityNormal is the default level, showing standard command output
	VerbosityNormal
	// VerbosityVerbose shows more detailed information about operations
	VerbosityVerbose
	// VerbosityDebug shows the most detailed information, including internal details
	VerbosityDebug
)

// String returns a string representation of the verbosity level
func (v VerbosityLevel) String() string {
	switch v {
	case VerbosityQuiet:
		return "quiet"
	case VerbosityNormal:
		return "normal"
	case VerbosityVerbose:
		return "verbose"
	case VerbosityDebug:
		return "debug"
	default:
		return "unknown"
	}
}

// ParseVerbosityLevel converts a string to a VerbosityLevel
func ParseVerbosityLevel(s string) VerbosityLevel {
	switch s {
	case "quiet":
		return VerbosityQuiet
	case "verbose":
		return VerbosityVerbose
	case "debug":
		return VerbosityDebug
	default:
		return VerbosityNormal
	}
}

// Verbosity provides a standardized interface for verbose output
type Verbosity struct {
	// Level is the current verbosity level
	Level VerbosityLevel
	// Writer is the output writer (defaults to os.Stdout)
	Writer io.Writer
}

// NewVerbosity creates a new Verbosity with the given level
func NewVerbosity(level VerbosityLevel) *Verbosity {
	return &Verbosity{
		Level:  level,
		Writer: os.Stdout,
	}
}

// IsQuiet returns true if the verbosity level is quiet
func (v *Verbosity) IsQuiet() bool {
	return v.Level == VerbosityQuiet
}

// IsVerbose returns true if verbose or debug mode is enabled
func (v *Verbosity) IsVerbose() bool {
	return v.Level >= VerbosityVerbose
}

// IsDebug returns true if debug mode is enabled
func (v *Verbosity) IsDebug() bool {
	return v.Level >= VerbosityDebug
}

// Print prints a message at the normal level
func (v *Verbosity) Print(format string, args ...interface{}) {
	if v.Level >= VerbosityNormal {
		fmt.Fprintf(v.Writer, format, args...)
	}
}

// Println prints a message with a trailing newline at the normal level
func (v *Verbosity) Println(format string, args ...interface{}) {
	if v.Level >= VerbosityNormal {
		fmt.Fprintf(v.Writer, format+"\n", args...)
	}
}

// Verbose prints a message at the verbose level
func (v *Verbosity) Verbose(format string, args ...interface{}) {
	if v.Level >= VerbosityVerbose {
		fmt.Fprintf(v.Writer, format, args...)
	}
}

// Verboseln prints a message with a trailing newline at the verbose level
func (v *Verbosity) Verboseln(format string, args ...interface{}) {
	if v.Level >= VerbosityVerbose {
		fmt.Fprintf(v.Writer, format+"\n", args...)
	}
}

// Debug prints a message at the debug level
func (v *Verbosity) Debug(format string, args ...interface{}) {
	if v.Level >= VerbosityDebug {
		fmt.Fprintf(v.Writer, "[DEBUG] "+format, args...)
	}
}

// Debugln prints a message with a trailing newline at the debug level
func (v *Verbosity) Debugln(format string, args ...interface{}) {
	if v.Level >= VerbosityDebug {
		fmt.Fprintf(v.Writer, "[DEBUG] "+format+"\n", args...)
	}
}

// ProgressStart initializes a progress display if not in quiet mode
func (v *Verbosity) ProgressStart(format string, args ...interface{}) {
	if v.Level >= VerbosityNormal && v.Level < VerbosityVerbose {
		fmt.Fprintf(v.Writer, format, args...)
	}
}

// ProgressUpdate updates a progress display if not in quiet mode
func (v *Verbosity) ProgressUpdate(format string, args ...interface{}) {
	if v.Level >= VerbosityNormal && v.Level < VerbosityVerbose {
		fmt.Fprintf(v.Writer, "\r"+format, args...)
	}
}

// ProgressFinish completes a progress display if not in quiet mode
func (v *Verbosity) ProgressFinish() {
	if v.Level >= VerbosityNormal && v.Level < VerbosityVerbose {
		fmt.Fprintln(v.Writer)
	}
}