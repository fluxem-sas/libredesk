// Package idempotency stores gateway idempotency keys so retries of ticket
// creation requests return the same conversation instead of creating duplicates.
package idempotency

import (
	"database/sql"
	"embed"
	"fmt"
	"time"

	"github.com/abhinavxd/libredesk/internal/dbutil"
	"github.com/jmoiron/sqlx"
	"github.com/zerodha/logf"
)

var (
	//go:embed queries.sql
	efs embed.FS
)

// Manager handles idempotency key storage and lookup.
type Manager struct {
	q  queries
	lo *logf.Logger
}

type queries struct {
	GetIdempotencyResponse    *sqlx.Stmt `query:"get-idempotency-response"`
	InsertIdempotencyResponse *sqlx.Stmt `query:"insert-idempotency-response"`
	DeleteOldIdempotencyKeys  *sqlx.Stmt `query:"delete-old-idempotency-keys"`
}

// New initializes a new idempotency Manager.
func New(db *sqlx.DB, lo *logf.Logger) (*Manager, error) {
	var q queries
	if err := dbutil.ScanSQLFile("queries.sql", &q, db, efs); err != nil {
		return nil, err
	}
	return &Manager{q: q, lo: lo}, nil
}

// Get returns the stored conversation UUID for an idempotency key.
// An empty string means no entry was found.
func (m *Manager) Get(applicationID int, key string) (string, error) {
	var uuid string
	if err := m.q.GetIdempotencyResponse.QueryRow(applicationID, key).Scan(&uuid); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return uuid, nil
}

// Set stores a conversation UUID for an idempotency key. It is safe to call
// when a key already exists; the existing value is kept.
func (m *Manager) Set(applicationID int, key, requestPath, conversationUUID string) error {
	_, err := m.q.InsertIdempotencyResponse.Exec(applicationID, key, requestPath, conversationUUID)
	return err
}

// Cleanup removes idempotency keys older than the given duration.
func (m *Manager) Cleanup(olderThan time.Duration) error {
	_, err := m.q.DeleteOldIdempotencyKeys.Exec(fmt.Sprintf("%d", int64(olderThan.Seconds())))
	return err
}
