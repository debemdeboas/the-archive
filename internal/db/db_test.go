package db

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

const failedToInitDB = "Failed to initialize database: %v"

const select1 = `SELECT 1`
const insertUserUsername = `INSERT INTO users (id, username) VALUES (?, ?)`

const testEmail = "test@example.com"

func TestSetLogger(t *testing.T) {
	logger := zerolog.New(os.Stdout).Level(zerolog.InfoLevel)
	SetLogger(logger)

	// Verify logger is set (we can't easily compare loggers directly)
	// This test mainly ensures the function doesn't panic
}

func TestNewSQLite(t *testing.T) {
	db := NewSQLite()

	if db == nil {
		t.Fatal("Expected non-nil SQLite instance")
	}

	if db.conn != nil {
		t.Error("Expected connection to be nil initially")
	}
}

func TestSQLiteBasicOperations(t *testing.T) {
	// Set up logger to reduce test output
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	SetLogger(logger)

	// Create temporary database file
	dbFile, err := os.CreateTemp("", "test-db-*.sqlite")
	if err != nil {
		t.Fatalf("Failed to create temp db file: %v", err)
	}
	defer os.Remove(dbFile.Name())
	dbFile.Close()

	// Note: We can't easily change the hardcoded path in InitDB,
	// so we'll work with the actual database file

	db := NewSQLite()
	defer db.Close()

	t.Run("InitDB creates tables", func(t *testing.T) {
		err := db.InitDB()
		if err != nil {
			t.Fatalf(failedToInitDB, err)
		}

		// Verify connection is established
		if db.Get() == nil {
			t.Error("Expected database connection to be established")
		}

		// Test that we can ping the database
		if err := db.Get().Ping(); err != nil {
			t.Errorf("Failed to ping database: %v", err)
		}
	})

	t.Run("Verify tables are created", func(t *testing.T) {
		// Check that expected tables exist
		tables := []string{"users", "drafts", "posts"}

		for _, table := range tables {
			query := "SELECT name FROM sqlite_master WHERE type='table' AND name=?"
			rows, err := db.Query(query, table)
			if err != nil {
				t.Errorf("Failed to query for table %s: %v", table, err)
				continue
			}

			if !rows.Next() {
				t.Errorf("Expected table %s to exist", table)
			}
			rows.Close()
		}
	})

	t.Run("Verify table schemas", func(t *testing.T) {
		// Test users table schema
		rows, err := db.Query("PRAGMA table_info(users)")
		if err != nil {
			t.Fatalf("Failed to get users table info: %v", err)
		}
		defer rows.Close()

		userColumns := make(map[string]bool)
		for rows.Next() {
			var cid int
			var name, dataType string
			var notNull, pk int
			var defaultValue sql.NullString

			err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk)
			if err != nil {
				t.Errorf("Failed to scan column info: %v", err)
				continue
			}
			userColumns[name] = true
		}

		expectedUserColumns := []string{"id", "username", "email", "created_at"}
		for _, col := range expectedUserColumns {
			if !userColumns[col] {
				t.Errorf("Expected users table to have column %s", col)
			}
		}

		// Test posts table schema
		rows, err = db.Query("PRAGMA table_info(posts)")
		if err != nil {
			t.Fatalf("Failed to get posts table info: %v", err)
		}
		defer rows.Close()

		postColumns := make(map[string]bool)
		for rows.Next() {
			var cid int
			var name, dataType string
			var notNull, pk int
			var defaultValue sql.NullString

			err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk)
			if err != nil {
				t.Errorf("Failed to scan column info: %v", err)
				continue
			}
			postColumns[name] = true
		}

		expectedPostColumns := []string{"id", "title", "content", "md_content_hash", "modified_at", "user_id", "created_at"}
		for _, col := range expectedPostColumns {
			if !postColumns[col] {
				t.Errorf("Expected posts table to have column %s", col)
			}
		}
	})

	t.Run("Foreign keys are enabled", func(t *testing.T) {
		rows, err := db.Query("PRAGMA foreign_keys")
		if err != nil {
			t.Fatalf("Failed to check foreign keys: %v", err)
		}
		defer rows.Close()

		if !rows.Next() {
			t.Fatal("Expected foreign keys pragma result")
		}

		var foreignKeysEnabled int
		err = rows.Scan(&foreignKeysEnabled)
		if err != nil {
			t.Fatalf("Failed to scan foreign keys result: %v", err)
		}

		if foreignKeysEnabled != 1 {
			t.Error("Expected foreign keys to be enabled")
		}
	})
}

func TestSQLiteQueryAndExec(t *testing.T) {
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	SetLogger(logger)

	db := NewSQLite()
	defer db.Close()

	err := db.InitDB()
	if err != nil {
		t.Fatalf(failedToInitDB, err)
	}

	t.Run("Exec inserts data", func(t *testing.T) {
		// Insert test user with unique ID
		userID := "test-user-exec-" + t.Name()
		username := "testuser-exec-" + t.Name()
		result, err := db.Exec("INSERT INTO users (id, username, email) VALUES (?, ?, ?)",
			userID, username, testEmail)
		if err != nil {
			t.Fatalf("Failed to insert user: %v", err)
		}

		// Check rows affected
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			t.Errorf("Failed to get rows affected: %v", err)
		}
		if rowsAffected != 1 {
			t.Errorf("Expected 1 row affected, got %d", rowsAffected)
		}
	})

	t.Run("Query retrieves data", func(t *testing.T) {
		// Insert unique user for this test
		userID := "test-user-query-" + t.Name()
		username := "testuser-query-" + t.Name()
		_, err := db.Exec("INSERT INTO users (id, username, email) VALUES (?, ?, ?)",
			userID, username, testEmail)
		if err != nil {
			t.Fatalf("Failed to insert user for query test: %v", err)
		}

		// Query the inserted user
		rows, err := db.Query("SELECT id, username, email FROM users WHERE id = ?", userID)
		if err != nil {
			t.Fatalf("Failed to query user: %v", err)
		}
		defer rows.Close()

		if !rows.Next() {
			t.Fatal("Expected to find inserted user")
		}

		var id, queriedUsername, email string
		err = rows.Scan(&id, &queriedUsername, &email)
		if err != nil {
			t.Fatalf("Failed to scan user data: %v", err)
		}

		if id != userID {
			t.Errorf("Expected id %q, got %s", userID, id)
		}
		if queriedUsername != username {
			t.Errorf("Expected username %q, got %s", username, queriedUsername)
		}
		if email != testEmail {
			t.Errorf("Expected email 'test@example.com', got %s", email)
		}
	})

	t.Run("Insert and query posts", func(t *testing.T) {
		// Insert test post with unique ID
		postID := "test-post-" + t.Name()
		userID := "test-user-for-post-" + t.Name()

		// First insert a user for the post
		_, err := db.Exec(insertUserUsername, userID, "user-"+t.Name())
		if err != nil {
			t.Fatalf("Failed to insert user for post test: %v", err)
		}

		// Insert test post
		_, err = db.Exec(`INSERT INTO posts (id, title, content, md_content_hash, user_id) 
			VALUES (?, ?, ?, ?, ?)`,
			postID, "Test Post", []byte("# Test Content"), "hash123", userID)
		if err != nil {
			t.Fatalf("Failed to insert post: %v", err)
		}

		// Query the post
		rows, err := db.Query("SELECT id, title, content FROM posts WHERE id = ?", postID)
		if err != nil {
			t.Fatalf("Failed to query post: %v", err)
		}
		defer rows.Close()

		if !rows.Next() {
			t.Fatal("Expected to find inserted post")
		}

		var id, title string
		var content []byte
		err = rows.Scan(&id, &title, &content)
		if err != nil {
			t.Fatalf("Failed to scan post data: %v", err)
		}

		if id != postID {
			t.Errorf("Expected id %q, got %s", postID, id)
		}
		if title != "Test Post" {
			t.Errorf("Expected title 'Test Post', got %s", title)
		}
		if string(content) != "# Test Content" {
			t.Errorf("Expected content '# Test Content', got %s", string(content))
		}
	})

	t.Run("Insert and query drafts", func(t *testing.T) {
		// Insert test draft with unique ID
		draftID := "test-draft-" + t.Name()
		userID := "test-user-for-draft-" + t.Name()

		// First insert a user for the draft
		_, err := db.Exec(insertUserUsername, userID, "user-"+t.Name())
		if err != nil {
			t.Fatalf("Failed to insert user for draft test: %v", err)
		}

		// Insert test draft
		_, err = db.Exec(`INSERT INTO drafts (id, title, content, user_id) 
			VALUES (?, ?, ?, ?)`,
			draftID, "Test Draft", []byte("Draft content"), userID)
		if err != nil {
			t.Fatalf("Failed to insert draft: %v", err)
		}

		// Query the draft
		rows, err := db.Query("SELECT id, title FROM drafts WHERE id = ?", draftID)
		if err != nil {
			t.Fatalf("Failed to query draft: %v", err)
		}
		defer rows.Close()

		if !rows.Next() {
			t.Fatal("Expected to find inserted draft")
		}

		var id, title string
		err = rows.Scan(&id, &title)
		if err != nil {
			t.Fatalf("Failed to scan draft data: %v", err)
		}

		if id != draftID {
			t.Errorf("Expected id %q, got %s", draftID, id)
		}
		if title != "Test Draft" {
			t.Errorf("Expected title 'Test Draft', got %s", title)
		}
	})
}

func TestSQLiteErrorHandling(t *testing.T) {
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	SetLogger(logger)

	t.Run("Query on uninitialized database", func(t *testing.T) {
		db := NewSQLite()
		defer db.Close()

		// Don't call InitDB() - connection will be nil
		// This should panic or error, but let's handle it gracefully
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic when querying uninitialized database")
			}
		}()

		db.Query(select1) // This will panic due to nil connection
	})

	t.Run("Exec on uninitialized database", func(t *testing.T) {
		db := NewSQLite()
		defer db.Close()

		// Don't call InitDB() - connection will be nil
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic when executing on uninitialized database")
			}
		}()

		db.Exec(select1) // This will panic due to nil connection
	})

	t.Run("Invalid SQL query", func(t *testing.T) {
		db := NewSQLite()
		defer db.Close()

		err := db.InitDB()
		if err != nil {
			t.Fatalf(failedToInitDB, err)
		}

		_, err = db.Query("INVALID SQL SYNTAX")
		if err == nil {
			t.Error("Expected error for invalid SQL")
		}
	})

	t.Run("Invalid SQL exec", func(t *testing.T) {
		db := NewSQLite()
		defer db.Close()

		err := db.InitDB()
		if err != nil {
			t.Fatalf(failedToInitDB, err)
		}

		_, err = db.Exec("INVALID SQL SYNTAX")
		if err == nil {
			t.Error("Expected error for invalid SQL")
		}
	})

	t.Run("Constraint violation", func(t *testing.T) {
		db := NewSQLite()
		defer db.Close()

		err := db.InitDB()
		if err != nil {
			t.Fatalf(failedToInitDB, err)
		}

		// Insert user with unique username for this test
		testUsername := "constraint-test-" + t.Name()
		_, err = db.Exec(insertUserUsername, "user1-"+t.Name(), testUsername)
		if err != nil {
			t.Fatalf("Failed to insert first user: %v", err)
		}

		// Try to insert another user with same username (should violate UNIQUE constraint)
		_, err = db.Exec(insertUserUsername, "user2-"+t.Name(), testUsername)
		if err == nil {
			t.Error("Expected constraint violation error for duplicate username")
		}
		if !strings.Contains(err.Error(), "UNIQUE") && !strings.Contains(err.Error(), "constraint") {
			t.Errorf("Expected UNIQUE constraint error, got: %v", err)
		}
	})
}

func TestSQLiteClose(t *testing.T) {
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	SetLogger(logger)

	t.Run("Close initialized database", func(t *testing.T) {
		db := NewSQLite()

		err := db.InitDB()
		if err != nil {
			t.Fatalf(failedToInitDB, err)
		}

		err = db.Close()
		if err != nil {
			t.Errorf("Failed to close database: %v", err)
		}

		// Verify connection is closed by trying to ping
		if db.Get() != nil {
			err = db.Get().Ping()
			if err == nil {
				t.Error("Expected connection to be closed")
			}
		}
	})

	t.Run("Close uninitialized database", func(t *testing.T) {
		db := NewSQLite()

		// Don't call InitDB()
		err := db.Close()
		if err != nil {
			t.Errorf("Expected no error closing uninitialized database, got: %v", err)
		}
	})

	t.Run("Close database twice", func(t *testing.T) {
		db := NewSQLite()

		err := db.InitDB()
		if err != nil {
			t.Fatalf(failedToInitDB, err)
		}

		err = db.Close()
		if err != nil {
			t.Errorf("Failed to close database first time: %v", err)
		}

		err = db.Close()
		if err != nil {
			t.Errorf("Failed to close database second time: %v", err)
		}
	})
}

func TestSQLiteGet(t *testing.T) {
	db := NewSQLite()
	defer db.Close()

	t.Run("Get before init returns nil", func(t *testing.T) {
		conn := db.Get()
		if conn != nil {
			t.Error("Expected nil connection before initialization")
		}
	})

	t.Run("Get after init returns connection", func(t *testing.T) {
		err := db.InitDB()
		if err != nil {
			t.Fatalf(failedToInitDB, err)
		}

		conn := db.Get()
		if conn == nil {
			t.Error("Expected non-nil connection after initialization")
		}

		// Verify it's a working connection
		err = conn.Ping()
		if err != nil {
			t.Errorf("Connection ping failed: %v", err)
		}
	})
}

func TestDbInterface(t *testing.T) {
	// Verify SQLite implements Db interface
	var _ DB = (*SQLite)(nil)

	// Test interface methods work
	db := NewSQLite()
	defer db.Close()

	// Test interface method calls
	err := db.InitDB()
	if err != nil {
		t.Fatalf("Interface InitDB failed: %v", err)
	}

	if db.Get() == nil {
		t.Error("Interface Get returned nil")
	}

	_, err = db.Query(select1)
	if err != nil {
		t.Errorf("Interface Query failed: %v", err)
	}

	_, err = db.Exec(select1)
	if err != nil {
		t.Errorf("Interface Exec failed: %v", err)
	}

	err = db.Close()
	if err != nil {
		t.Errorf("Interface Close failed: %v", err)
	}
}

func TestDatabaseCreationWithCustomPath(t *testing.T) {
	// Note: This test demonstrates the limitation that the database path is hardcoded
	// In a real refactor, we'd want to make the path configurable

	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	SetLogger(logger)

	t.Run("Database file is created", func(t *testing.T) {
		// Clean up any existing database file
		os.Remove("./database.db")
		defer os.Remove("./database.db")

		db := NewSQLite()
		defer db.Close()

		err := db.InitDB()
		if err != nil {
			t.Fatalf(failedToInitDB, err)
		}

		// Check that the database file was created
		if _, err := os.Stat("./database.db"); os.IsNotExist(err) {
			t.Error("Expected database file to be created")
		}
	})
}

func BenchmarkSQLiteOperations(b *testing.B) {
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	SetLogger(logger)

	// Clean up
	os.Remove("./database.db")
	defer os.Remove("./database.db")

	db := NewSQLite()
	defer db.Close()

	err := db.InitDB()
	if err != nil {
		b.Fatalf(failedToInitDB, err)
	}

	b.Run("Insert", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := db.Exec(insertUserUsername,
				b.Name()+"-user-"+string(rune(i)), "user"+string(rune(i)))
			if err != nil {
				b.Errorf("Failed to insert user: %v", err)
			}
		}
	})

	b.Run("Query", func(b *testing.B) {
		// Pre-populate some data
		for i := 0; i < 100; i++ {
			db.Exec("INSERT OR IGNORE INTO users (id, username) VALUES (?, ?)",
				"bench-user-"+string(rune(i)), "benchuser"+string(rune(i)))
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rows, err := db.Query("SELECT id, username FROM users LIMIT 10")
			if err != nil {
				b.Errorf("Failed to query users: %v", err)
				continue
			}

			// Process results
			for rows.Next() {
				var id, username string
				rows.Scan(&id, &username)
			}
			rows.Close()
		}
	})
}
