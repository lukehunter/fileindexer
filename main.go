package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

const createTableQuery = `
CREATE TABLE IF NOT EXISTS file_hashes (
    id SERIAL PRIMARY KEY,
    filepath TEXT NOT NULL UNIQUE,
    hash TEXT NOT NULL,
    size INTEGER NOT NULL,
    file_timestamp TIMESTAMP NOT NULL,
    hash_calculated_timestamp TIMESTAMP NOT NULL
);
`

func main() {
	// Main function to coordinate file hashing and database operations
	directory := flag.String("directory", "", "Target directory to process")                                                            // Directory to scan for files
	dbName := flag.String("dbname", "", "PostgreSQL database name")                                                                     // Name of the database to connect to
	dbUser := flag.String("dbuser", os.Getenv("DB_USER"), "PostgreSQL user")                                                            // Database user (default from environment variable), "PostgreSQL user")
	dbHost := flag.String("dbhost", os.Getenv("DB_HOST"), "PostgreSQL host")                                                            // Database host (default from environment variable), "PostgreSQL host")
	dbPort := flag.String("dbport", os.Getenv("DB_PORT"), "PostgreSQL port")                                                            // Database port (default from environment variable), "PostgreSQL port")
	outputFile := flag.String("output", fmt.Sprintf("%s_results.csv", time.Now().Format("2006-01-02T15.04.05.000")), "Output CSV file") // CSV file to save results.Format("2006-01-02T15.04.05.000")), "Output CSV file")
	prefix := flag.String("prefix", "", "Prefix to remove from the file path when storing in the database")                             // Optional prefix to remove from file paths
	flag.Parse()

	if *directory == "" || *dbName == "" {
		log.Fatalf("Usage: --directory <target_directory> --dbname <postgres_db_name> [--dbuser <user>] [--dbhost <host>] [--dbport <port>] [--output <output_file>]")
	}

	dbPassword := os.Getenv("DB_PASSWORD") // Fetch database password from environment variable
	if dbPassword == "" {
		fmt.Print("Enter database password: ")
		var inputPassword string
		fmt.Scanln(&inputPassword)
		dbPassword = inputPassword
	}

	connectionString := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", *dbHost, *dbPort, *dbUser, dbPassword, *dbName) // Build database connection string
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Create table if it doesn't exist
	if _, err := db.Exec(createTableQuery); err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	// Open output CSV file
	file, err := os.Create(*outputFile)
	if err != nil {
		log.Fatalf("Failed to create output file: %v", err)
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush() // Ensure CSV data is written to disk

	if err := writer.Write([]string{"filepath", "hash", "size", "status"}); err != nil {
		log.Fatalf("Failed to write CSV header: %v", err)
	}

	// Concurrency setup
	sem := make(chan struct{}, 8) // Semaphore to limit concurrency to 8 workers
	var wg sync.WaitGroup
	errCh := make(chan error) // Channel to collect errors from goroutines

	// Walk through files
	err = filepath.Walk(*directory, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			log.Printf("Error accessing %s: %v", path, walkErr)
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}

		storedPath := path
		if *prefix != "" && len(path) > len(*prefix) && path[:len(*prefix)] == *prefix {
			storedPath = path[len(*prefix):]
		}

		sem <- struct{}{}
		wg.Add(1)
		go func(path, storedPath string) {
			defer func() {
				<-sem
				wg.Done()
			}()

			hash, size, status, err := processFile(path, storedPath, db)
			if err != nil {
				log.Printf("Skipping file %s due to error: %v", path, err) // Log error for the file but continue processing other files
				if writeErr := writer.Write([]string{storedPath, "", "-1", fmt.Sprintf("error: %v", err)}); writeErr != nil {
					log.Printf("Failed to write error to CSV for file %s: %v", path, writeErr)
				}
				return
			}
			log.Printf("Path: %s Hash: %s, Size: %d, Status: %s", path, hash, size, status)
			if writeErr := writer.Write([]string{storedPath, hash, fmt.Sprintf("%d", size), status}); writeErr != nil {
				log.Printf("Failed to write result to CSV for file %s: %v", path, writeErr)
			}
		}(path, storedPath)
		return nil
	})
	if err != nil {
		log.Printf("Error walking through files: %v", err)
	}

	wg.Wait()
	close(errCh)

	log.Printf("SHA256 hash calculation and storage completed. Results saved to %s", *outputFile)
}

func processFile(path, storedPath string, db *sql.DB) (string, int64, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", -1, "", fmt.Errorf("failed to open file %s: %v", path, err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return "", -1, "", fmt.Errorf("failed to stat file %s: %v", path, err)
	}

	size := fileInfo.Size()
	fileTimestamp := fileInfo.ModTime()

	var dbHash string
	var dbSize int64
	err = db.QueryRow("SELECT hash, size FROM file_hashes WHERE filepath = $1", storedPath).Scan(&dbHash, &dbSize)
	if errors.Is(err, sql.ErrNoRows) {
		hasher := sha256.New()
		if _, err := io.Copy(hasher, file); err != nil {
			return "", -1, "", fmt.Errorf("failed to hash file %s: %v", path, err)
		}
		hash := fmt.Sprintf("%x", hasher.Sum(nil))
		for {
			_, err = db.Exec("INSERT INTO file_hashes (filepath, hash, size, file_timestamp, hash_calculated_timestamp) VALUES ($1, $2, $3, $4, $5)", storedPath, hash, size, fileTimestamp, time.Now())
			if err == nil {
				break
			}
			log.Printf("Retrying INSERT for %s: %v", path, err)
			time.Sleep(1 * time.Second)
		}
		return hash, size, "new", nil
	} else if err != nil {
		return "", -1, "", fmt.Errorf("failed to query database for %s: %v", storedPath, err)
	}

	if size != dbSize {
		hash := fmt.Sprintf("%x", sha256.New().Sum(nil))
		for {
			_, err = db.Exec("UPDATE file_hashes SET hash = $1, size = $2, file_timestamp = $3, hash_calculated_timestamp = $4 WHERE filepath = $5", hash, size, fileTimestamp, time.Now(), storedPath)
			if err == nil {
				break
			}
			log.Printf("Retrying UPDATE for %s: %v", storedPath, err)
			time.Sleep(1 * time.Second)
		}
		return hash, size, "changed", nil
	}
	return dbHash, dbSize, "existing", nil
}
