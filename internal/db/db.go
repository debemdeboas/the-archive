package db

import (
	"database/sql"

	"github.com/rs/zerolog"
)

type Db interface {
	InitDb() error

	Get() *sql.DB
	Close() error

	Query(query string, args ...interface{}) (*sql.Rows, error)
	Exec(query string, args ...interface{}) (sql.Result, error)
}

var dbLogger zerolog.Logger

func SetLogger(l zerolog.Logger) {
	dbLogger = l
}
