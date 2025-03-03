package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

type SQLite struct {
	conn *sql.DB
}

func NewSQLite() *SQLite {
	return &SQLite{
		conn: nil,
	}
}

func (s *SQLite) InitDb() error {
	var err error
	s.conn, err = sql.Open("sqlite3", "./database.db")
	if err != nil {
		return err
	}

	// We've removed the user_id foreign key from the posts/drafts table for now.
	// FOREIGN KEY(user_id) REFERENCES users(id)

	// Comments are also removed from the schema.
	// CREATE TABLE IF NOT EXISTS comments (
	//     id TEXT PRIMARY KEY,
	//     post_id TEXT,
	//     user_id TEXT,
	//     comment TEXT,
	//     created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	//     FOREIGN KEY(post_id) REFERENCES posts(id),
	//     FOREIGN KEY(user_id) REFERENCES users(id)
	// );

	res, err := s.conn.Exec(`
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    username TEXT UNIQUE,
    email TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS drafts (
    id TEXT PRIMARY KEY,
    title TEXT,
    content BLOB,
    user_id TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS posts (
    id TEXT PRIMARY KEY,
    title TEXT,
    content BLOB,
    md_content_hash TEXT,
    modified_at DATETIME,
    user_id TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);`)

	dbLogger.Info().Any("db_result", res).Msg("Database initialized")
	return err
}

func (s *SQLite) Get() *sql.DB {
	return s.conn
}

func (s *SQLite) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

func (s *SQLite) Query(query string, args ...interface{}) (*sql.Rows, error) {
	dbLogger.Info().Str("query", query).Msg("Query")
	return s.conn.Query(query, args...)
}

func (s *SQLite) Exec(query string, args ...interface{}) (sql.Result, error) {
	dbLogger.Info().Str("query", query).Msg("Exec")
	return s.conn.Exec(query, args...)
}
