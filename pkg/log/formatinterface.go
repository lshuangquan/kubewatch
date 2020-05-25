package log

import (
	"fmt"
	"path/filepath"
	"runtime"
	"time"
)

const (
	// Runtime caller depth
	depth = 3
)

// Formatter interface
type Formatter interface {
	GetPrefix(lvl level) string
	Format(lvl level, v ...interface{}) []interface{}
	GetSuffix(lvl level) string
}

// Returns header including filename and line number
func header() string {
	_, fn, line, ok := runtime.Caller(depth)
	if !ok {
		fn = "???"
		line = 1
	}
	prefix := time.Now().Format("2006/01/02 15:04:05") + fmt.Sprintf(" %s:%d ", filepath.Base(fn), line)
	return prefix
}
