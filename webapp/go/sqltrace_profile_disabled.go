//go:build !sqlite_trace && !trace
// +build !sqlite_trace,!trace

package isuports

import (
	"fmt"
	"io"
)

func initializeSQLiteProfiler(path string) (string, io.Closer, error) {
	return "", nil, fmt.Errorf("ISUCON_SQLITE_PROFILE_FILE requires building with -tags sqlite_trace")
}
