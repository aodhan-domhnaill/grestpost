package api

import "github.com/jmoiron/sqlx"

type rowsInterface interface {
	MapScan(dest map[string]interface{}) error
	Next() bool
	Close() error
	Scan(dest ...interface{}) error
}

// databaseInterface - describes the methods used
type databaseInterface interface {
	NamedQuery(query string, arg interface{}) (rowsInterface, error)
	Beginx() (txInterface, error)
}

type txInterface interface {
	NamedQuery(query string, arg interface{}) (rowsInterface, error)
	Rollback() error
}

// Wrapper that implements databaseInterface
type databaseBackend struct {
	db *sqlx.DB
}

type txBackend struct {
	txn *sqlx.Tx
}

// API - API object
type API struct {
	sql databaseInterface
}

func (db databaseBackend) NamedQuery(query string, arg interface{}) (rowsInterface, error) {
	rows, err := db.db.NamedQuery(query, arg)
	return rowsInterface(rows), err
}

func (db databaseBackend) Beginx() (txInterface, error) {
	txn, err := db.db.Beginx()
	return txBackend{txn}, err
}

func (txn txBackend) NamedQuery(query string, arg interface{}) (rowsInterface, error) {
	rows, err := txn.txn.NamedQuery(query, arg)
	return rowsInterface(rows), err
}

func (txn txBackend) Rollback() error {
	return txn.txn.Rollback()
}
