package db

import "database/sql"

type Db interface {
	InitDb() error

	Get() *sql.DB
	Close() error

	Query(query string, args ...interface{}) (*sql.Rows, error)
	Exec(query string, args ...interface{}) (sql.Result, error)
}
