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
	"strconv"
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
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
`

func main() {
	directory := flag.String("directory", "", "Target directory to process")
	dbName := flag.String("dbname", "", "PostgreSQL database name")
	dbUser := flag.String("dbuser", os.Getenv("DB_USER"), "PostgreSQL user")
	dbHost := flag.String("dbhost", os.Getenv("DB_HOST"), "PostgreSQL host")
	dbPort := flag.String("dbport", os.Getenv("DB_PORT"), "PostgreSQL port")
	outputFile := flag.String("output", fmt.Sprintf("%s_results.csv", time.Now().Format("2006-01-02T15.04.05.000")), "Output CSV file")
	flag.Parse()

	if *directory == "" || *dbName == "" {
		log.Fatalf("Usage: --directory <target_directory> --dbname <postgres_db_name> [--dbuser <user>] [--dbhost <host>] [--dbport <port>] [--output <output_file>]")
	}

	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		fmt.Print("Enter database password: ")
		var inputPassword string
		fmt.Scanln(&inputPassword)
		dbPassword = inputPassword
	}

	connectionString := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", *dbHost, *dbPort, *dbUser, dbPassword, *dbName)
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
	defer writer.Flush()

	if err := writer.Write([]string{"filepath", "hash", "size", "status"}); err != nil {
		log.Fatalf("Failed to write CSV header: %v", err)
	}

	// Set up concurrency
	sem := make(chan struct{}, 8) // Limit to 8 concurrent workers
	var wg sync.WaitGroup
	errCh := make(chan error, 1)

	// Walk through files and process them in parallel
	err = filepath.Walk(*directory, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !info.Mode().IsRegular() {
			return nil
		}

		sem <- struct{}{}
		wg.Add(1)
		go func(path string) {
			defer func() {
				<-sem
				wg.Done()
			}()
			log.Printf("Processing: %s", path)
			hash, size, status, err := processFile(path, db)
			if err != nil {
				select {
				case errCh <- err:
				default:
				}
				return
			}
			if writeErr := writer.Write([]string{path, hash, strconv.FormatInt(size, 10), status}); writeErr != nil {
				select {
				case errCh <- writeErr:
				default:
				}
			}
		}(path)
		return nil
	})
	if err != nil {
		log.Fatalf("Error walking through files: %v", err)
	}

	wg.Wait()
	close(errCh)
	if len(errCh) > 0 {
		log.Fatalf("Error processing files: %v", <-errCh)
	}

	log.Printf("SHA256 hash calculation and storage completed. Results saved to %s", *outputFile)
}

func processFile(path string, db *sql.DB) (string, int64, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", -1, "", err
	}
	defer file.Close()

	hasher := sha256.New()
	size, err := io.Copy(hasher, file)
	if err != nil {
		return "", -1, "", err
	}
	hash := fmt.Sprintf("%x", hasher.Sum(nil))

	var dbHash string
	var dbSize int64
	err = db.QueryRow("SELECT hash, size FROM file_hashes WHERE filepath = $1", path).Scan(&dbHash, &dbSize)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Insert new record
			for {
				_, err = db.Exec("INSERT INTO file_hashes (filepath, hash, size) VALUES ($1, $2, $3)", path, hash, size)
				if err == nil {
					break
				}
				log.Printf("Retrying INSERT for %s due to error: %v", path, err)
				time.Sleep(1 * time.Second)
			}
			if err != nil {
				return "", -1, "", err
			}
			return hash, size, "new", nil
		}
		return "", -1, "", err
	}

	if size != dbSize {
		// Update record
		for {
			_, err = db.Exec("UPDATE file_hashes SET hash = $1, size = $2 WHERE filepath = $3", hash, size, path)
			if err == nil {
				break
			}
			log.Printf("Retrying UPDATE for %s due to error: %v", path, err)
			time.Sleep(1 * time.Second)
		}
		if err != nil {
			return "", -1, "", err
		}
		return hash, size, "changed", nil
	}

	return dbHash, dbSize, "existing", nil
}
