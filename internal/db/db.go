// Package db provides database interface abstractions and connection management.
package db

import (
	"database/sql"

	"github.com/rs/zerolog"
)

type DB interface {
	InitDB() error

	Get() *sql.DB
	Close() error

	Query(query string, args ...interface{}) (*sql.Rows, error)
	Exec(query string, args ...interface{}) (sql.Result, error)
}

var dbLogger zerolog.Logger

func SetLogger(l zerolog.Logger) {
	dbLogger = l
}
