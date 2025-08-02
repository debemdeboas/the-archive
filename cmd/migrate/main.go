package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/debemdeboas/the-archive/internal/db"
	"github.com/debemdeboas/the-archive/internal/model"
	"github.com/debemdeboas/the-archive/internal/repository"
	"github.com/debemdeboas/the-archive/internal/util"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// main is the entry point of the script, parsing flags and orchestrating the migration.
func main() {
	// Define command-line flags
	path := flag.String("path", "", "Path to the directory containing .md files")
	ownerID := flag.String("owner-id", "", "Owner user ID for the posts")
	flag.Parse()

	// Validate required flags
	if *path == "" || *ownerID == "" {
		log.Fatal("Both --path and --owner-id flags are required")
	}

	// Initialize the SQLite database and ensure tables exist
	DB := db.NewSQLite()
	DB.InitDB()

	// Create a repository instance to interact with the database
	repo := repository.NewDBPostRepository(DB)

	// Read all files from the specified directory
	files, err := os.ReadDir(*path)
	if err != nil {
		log.Fatalf("Error reading directory %s: %v", *path, err)
	}

	// Process each .md file
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".md") {
			err := processFile(*path, file, repo, *ownerID)
			if err != nil {
				log.Printf("Error processing file %s: %v", file.Name(), err)
				continue
			}
			log.Printf("Successfully saved post from file: %s", file.Name())
		}
	}
}

// processFile handles the migration of a single .md file to the database.
func processFile(dirPath string, file os.DirEntry, repo repository.PostRepository, ownerID string) error {
	filePath := filepath.Join(dirPath, file.Name())

	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	frontMatter := util.GetFrontMatter(content)

	// Determine the title: use front matter if available, otherwise use the file name
	title := strings.TrimSuffix(file.Name(), ".md")
	if frontMatter != nil && frontMatter.Title != "" {
		title = frontMatter.Title
	}

	// Get file metadata
	fileInfo, err := file.Info()
	if err != nil {
		return err
	}
	modTime := fileInfo.ModTime()

	// Set creation and modification dates
	createdDate := modTime.UTC()
	if frontMatter != nil {
		createdDate = frontMatter.Date.UTC()
	}

	modifiedDate := modTime.UTC()

	// Create a new post struct
	post := &model.Post{
		ID:           model.PostID(uuid.New().String()),
		Title:        title,
		Markdown:     content,
		CreatedDate:  createdDate,
		ModifiedDate: modifiedDate,
		Owner:        model.UserID(ownerID),
		Path:         "",
	}
	post.Path = string(post.ID)

	// Save the post to the database
	return repo.SavePost(post)
}
