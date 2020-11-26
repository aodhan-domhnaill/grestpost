package api

import (
	"database/sql"

	"github.com/jmoiron/sqlx"
)

type rowsInterface interface {
	MapScan(dest map[string]interface{}) error
	Next() bool
	Close() error
	Scan(dest ...interface{}) error
	Err() error
}

// databaseInterface - describes the methods used
type databaseInterface interface {
	NamedExec(query string, arg interface{}) (sql.Result, error)
	NamedQuery(query string, arg interface{}) (rowsInterface, error)
	Beginx() (txInterface, error)
}

type txInterface interface {
	NamedExec(query string, arg interface{}) (sql.Result, error)
	NamedQuery(query string, arg interface{}) (rowsInterface, error)
	Rollback() error
	Commit() error
}

// Wrapper that implements databaseInterface
type databaseBackend struct {
	db *sqlx.DB
}

type txBackend struct {
	txn *sqlx.Tx
}

func (db databaseBackend) NamedQuery(query string, arg interface{}) (rowsInterface, error) {
	rows, err := db.db.NamedQuery(query, arg)
	return rowsInterface(rows), err
}

func (db databaseBackend) NamedExec(query string, arg interface{}) (sql.Result, error) {
	return db.db.NamedExec(query, arg)
}

func (db databaseBackend) Beginx() (txInterface, error) {
	txn, err := db.db.Beginx()
	return txBackend{txn}, err
}

func (txn txBackend) NamedQuery(query string, arg interface{}) (rowsInterface, error) {
	rows, err := txn.txn.NamedQuery(query, arg)
	return rowsInterface(rows), err
}

func (txn txBackend) NamedExec(query string, arg interface{}) (sql.Result, error) {
	return txn.txn.NamedExec(query, arg)
}

func (txn txBackend) Rollback() error {
	return txn.txn.Rollback()
}

func (txn txBackend) Commit() error {
	return txn.txn.Commit()
}
