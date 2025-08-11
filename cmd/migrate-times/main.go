package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/debemdeboas/the-archive/internal/db"
	_ "github.com/mattn/go-sqlite3"
)

// parseFuzzyTime attempts to parse a timestamp string using multiple formats.
func parseFuzzyTime(timeStr string) (time.Time, error) {
	timeFormats := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05-07:00",
		time.RFC3339,
		"2006-01-02 15:04:05", // Added for cases without timezone info
	}

	var parsedTime time.Time
	var err error
	for _, format := range timeFormats {
		parsedTime, err = time.Parse(format, timeStr)
		if err == nil {
			return parsedTime.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("failed to parse time '%s' with any known format", timeStr)
}

// updateTimestamp updates a single timestamp in the database.
func updateTimestamp(db *sql.DB, id, column string, newTime time.Time) error {
	_, err := db.Exec(fmt.Sprintf("UPDATE posts SET %s = ? WHERE id = ?", column), newTime, id)
	return err
}

func main() {
	log.Println("Starting timestamp migration...")

	// Initialize database connection
	database := db.NewSQLite()
	if err := database.InitDB(); err != nil {
		log.Fatalf("Error initializing database: %v", err)
	}
	defer database.Close()

	sqlDB := database.Get()

	// Fetch all post timestamps
	rows, err := sqlDB.Query("SELECT id, created_at, modified_at FROM posts")
	if err != nil {
		log.Fatalf("Failed to query posts: %v", err)
	}
	defer rows.Close()

	type PostTime struct {
		ID         string
		CreatedAt  string
		ModifiedAt string
	}

	var posts []PostTime
	for rows.Next() {
		var p PostTime
		if err := rows.Scan(&p.ID, &p.CreatedAt, &p.ModifiedAt); err != nil {
			log.Printf("Failed to scan row: %v", err)
			continue
		}
		posts = append(posts, p)
	}

	if err := rows.Err(); err != nil {
		log.Fatalf("Error during row iteration: %v", err)
	}

	log.Printf("Found %d posts to process.", len(posts))

	// Process each post
	for _, p := range posts {
		// Process created_at
		createdAt, err := parseFuzzyTime(p.CreatedAt)
		if err != nil {
			log.Printf("ID %s: Could not parse created_at '%s': %v", p.ID, p.CreatedAt, err)
		} else {
			if err := updateTimestamp(sqlDB, p.ID, "created_at", createdAt); err != nil {
				log.Printf("ID %s: Failed to update created_at: %v", p.ID, err)
			} else {
				log.Printf("ID %s: Successfully updated created_at to %s", p.ID, createdAt.Format(time.RFC3339Nano))
			}
		}

		// Process modified_at
		modifiedAt, err := parseFuzzyTime(p.ModifiedAt)
		if err != nil {
			log.Printf("ID %s: Could not parse modified_at '%s': %v", p.ID, p.ModifiedAt, err)
		} else {
			if err := updateTimestamp(sqlDB, p.ID, "modified_at", modifiedAt); err != nil {
				log.Printf("ID %s: Failed to update modified_at: %v", p.ID, err)
			} else {
				log.Printf("ID %s: Successfully updated modified_at to %s", p.ID, modifiedAt.Format(time.RFC3339Nano))
			}
		}
	}

	log.Println("Timestamp migration complete.")
}
