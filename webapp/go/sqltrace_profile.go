//go:build sqlite_trace || trace
// +build sqlite_trace trace

package isuports

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mattn/go-sqlite3"
)

// PROFILE fires on completion/reset, after SELECT rows have been consumed.
// SQLite's reported runtime has millisecond resolution; sub-ms queries show 0.
func initializeSQLiteProfiler(path string) (string, io.Closer, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return "", nil, err
	}
	var outputMu sync.Mutex
	type statement struct {
		sql     string
		started time.Time
		rows    int64
	}
	sql.Register("sqlite3-with-profile", &sqlite3.SQLiteDriver{ConnectHook: func(conn *sqlite3.SQLiteConn) error {
		var mu sync.Mutex
		statements := make(map[uintptr]*statement)
		return conn.SetTrace(&sqlite3.TraceConfig{
			EventMask: sqlite3.TraceStmt | sqlite3.TraceProfile | sqlite3.TraceRow | sqlite3.TraceClose,
			Callback: func(info sqlite3.TraceInfo) int {
				mu.Lock()
				defer mu.Unlock()
				switch info.EventCode {
				case sqlite3.TraceStmt:
					if !strings.HasPrefix(info.StmtOrTrigger, "--") {
						statements[info.StmtHandle] = &statement{sql: info.StmtOrTrigger, started: time.Now()}
					}
				case sqlite3.TraceRow:
					if s := statements[info.StmtHandle]; s != nil {
						s.rows++
					}
				case sqlite3.TraceProfile:
					if s := statements[info.StmtHandle]; s != nil {
						delete(statements, info.StmtHandle)
						// MySQL-compatible envelope lets the existing slp processor aggregate SQLite.
						entry := fmt.Sprintf("# Time: %s\n# User@Host: sqlite[sqlite] @ localhost []\n# Query_time: %.9f Lock_time: 0.000000 Rows_sent: %d Rows_examined: 0\nSET timestamp=%d;\n%s;\n", s.started.UTC().Format(time.RFC3339Nano), float64(info.RunTimeNanosec)/1e9, s.rows, s.started.Unix(), strings.TrimSuffix(strings.TrimSpace(s.sql), ";"))
						outputMu.Lock()
						_, err := io.WriteString(file, entry)
						outputMu.Unlock()
						if err != nil {
							log.Printf("SQLite profile write failed: %v", err)
						}
					}
				case sqlite3.TraceClose:
					statements = make(map[uintptr]*statement)
				}
				return 0
			},
		})
	}})
	return "sqlite3-with-profile", file, nil
}
