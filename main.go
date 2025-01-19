package main

import (
	"crypto/md5"
	"database/sql"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

const createTableQuery = `
CREATE TABLE IF NOT EXISTS file_hashes (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    filepath TEXT NOT NULL UNIQUE,
    hash TEXT NOT NULL,
    size BIGINT NOT NULL,
    file_timestamp TIMESTAMP NOT NULL,
    hash_calculated_timestamp TIMESTAMP NOT NULL
);
`

type Config struct {
	Directory      string
	DbName         string
	DbUser         string
	DbHost         string
	DbPort         string
	DbPassword     string
	OutputFile     string
	Prefix         string
	ExcludeStrings []string
}

func parseFlags() Config {
	directory := flag.String("directory", "", "The target directory containing files to process for MD5 hash calculation. Required.")
	dbName := flag.String("dbname", "", "The name of the PostgreSQL database to store file hashes. Required.")
	dbUser := flag.String("dbuser", os.Getenv("DB_USER"), "The PostgreSQL username. Defaults to the DB_USER environment variable.")
	dbHost := flag.String("dbhost", os.Getenv("DB_HOST"), "The PostgreSQL host. Defaults to the DB_HOST environment variable.")
	dbPort := flag.String("dbport", os.Getenv("DB_PORT"), "The PostgreSQL port. Defaults to the DB_PORT environment variable.")
	outputFile := flag.String("output", fmt.Sprintf("%s_results.csv", time.Now().Format("2006-01-02T15.04.05.000")), "The path to the CSV file to output processing results. Defaults to a timestamped file in the current directory.")
	prefix := flag.String("prefix", "", "Optional prefix to remove from file paths when storing them in the database.")
	excludeStrings := flag.String("exclude", "", "Comma-separated list of strings. Skip processing files containing any of these strings in their path.")
	flag.Parse()

	if *directory == "" || *dbName == "" {
		log.Fatalf(`Usage: <command> --directory <target_directory> --dbname <postgres_db_name> [options]

This command scans a directory for files, computes their MD5 hashes, stores the hashes and metadata in a PostgreSQL database, and outputs a CSV summary.

Required Flags:
  --directory: The target directory to process.
  --dbname: The name of the PostgreSQL database.

Optional Flags:
  --dbuser: PostgreSQL username (default: DB_USER environment variable).
  --dbhost: PostgreSQL host (default: DB_HOST environment variable).
  --dbport: PostgreSQL port (default: DB_PORT environment variable).
  --output: Output CSV file path (default: timestamped file in the current directory).
  --prefix: Prefix to remove from file paths in the database.
  --exclude: Comma-separated strings to exclude certain file paths.`)
	}

	return Config{
		Directory:      *directory,
		DbName:         *dbName,
		DbUser:         *dbUser,
		DbHost:         *dbHost,
		DbPort:         *dbPort,
		OutputFile:     *outputFile,
		Prefix:         *prefix,
		ExcludeStrings: strings.Split(*excludeStrings, ","),
	}
}

func connectToDatabase(cfg Config) *sql.DB {
	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		fmt.Print("Enter database password: ")
		var inputPassword string
		fmt.Scanln(&inputPassword)
		dbPassword = inputPassword
	}

	connectionString := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.DbHost, cfg.DbPort, cfg.DbUser, dbPassword, cfg.DbName,
	)
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	return db
}

func createOutputWriter(outputFile string) (*csv.Writer, *os.File) {
	file, err := os.Create(outputFile)
	if err != nil {
		log.Fatalf("Failed to create output file: %v", err)
	}
	writer := csv.NewWriter(file)
	if err := writer.Write([]string{"filepath", "hash", "size", "status"}); err != nil {
		log.Fatalf("Failed to write CSV header: %v", err)
	}
	return writer, file
}

func processDirectory(cfg Config, db *sql.DB, writer *csv.Writer, writerMutex *sync.Mutex) {
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup

	err := filepath.Walk(cfg.Directory, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			log.Printf("Error accessing %s: %v", path, walkErr)
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}

		for _, exclude := range cfg.ExcludeStrings {
			if exclude != "" && strings.Contains(path, exclude) {
				log.Printf("Skipping file %s due to exclusion string: %s", path, exclude)
				return nil
			}
		}

		storedPath := path
		if cfg.Prefix != "" && strings.HasPrefix(path, cfg.Prefix) {
			storedPath = path[len(cfg.Prefix):]
		}

		sem <- struct{}{}
		wg.Add(1)
		go func(path, storedPath string) {
			defer func() {
				<-sem
				wg.Done()
			}()

			hash, size, status, err := processFile(path, storedPath, db)
			writerMutex.Lock()
			defer writerMutex.Unlock()

			if err != nil {
				log.Printf("Skipping file %s due to error: %v", path, err)
				if writeErr := writer.Write([]string{storedPath, "", "-1", fmt.Sprintf("error: %v", err)}); writeErr != nil {
					log.Printf("Failed to write error to CSV for file %s: %v", path, writeErr)
				}
				writer.Flush()
				return
			}

			log.Printf("Path: %s Hash: %s, Size: %d, Status: %s", path, hash, size, status)
			if writeErr := writer.Write([]string{storedPath, hash, fmt.Sprintf("%d", size), status}); writeErr != nil {
				log.Printf("Failed to write result to CSV for file %s: %v", path, writeErr)
			}
			writer.Flush()
		}(path, storedPath)
		return nil
	})

	if err != nil {
		log.Printf("Error walking through files: %v", err)
	}

	wg.Wait()
}

func main() {
	cfg := parseFlags()
	db := connectToDatabase(cfg)
	defer db.Close()

	log.Printf("Creating table if it doesn't exist")
	if _, err := db.Exec(createTableQuery); err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	writer, outputFile := createOutputWriter(cfg.OutputFile)
	defer func() {
		writer.Flush()
		outputFile.Close()
	}()

	writerMutex := &sync.Mutex{}
	processDirectory(cfg, db, writer, writerMutex)

	log.Printf("MD5 hash calculation and storage completed. Results saved to %s", cfg.OutputFile)
}

func processFile(path, storedPath string, db *sql.DB) (string, int64, string, error) {
	// Open the file for reading
	file, err := os.Open(path)
	if err != nil {
		return "", -1, "", fmt.Errorf("failed to open file %s: %v", path, err)
	}
	defer file.Close()

	// Retrieve file metadata
	size, fileTimestamp, err := getFileMetadata(file)
	if err != nil {
		return "", -1, "", fmt.Errorf("failed to retrieve metadata for file %s: %v", path, err)
	}

	// Check if the file exists in the database
	dbHash, dbSize, err := getDatabaseRecord(db, storedPath)
	if errors.Is(err, sql.ErrNoRows) {
		// If no record exists, hash and insert the file
		hash, err := hashFile(file)
		if err != nil {
			return "", -1, "", fmt.Errorf("failed to hash file %s: %v", path, err)
		}
		if err := insertFileRecord(db, storedPath, hash, size, fileTimestamp); err != nil {
			return "", -1, "", fmt.Errorf("failed to insert record for file %s: %v", path, err)
		}
		return hash, size, "new", nil
	} else if err != nil {
		return "", -1, "", fmt.Errorf("failed to query database for %s: %v", storedPath, err)
	}

	// Update the record if the size has changed
	if size != dbSize {
		hash, err := hashFile(file)
		if err != nil {
			return "", -1, "", fmt.Errorf("failed to hash file %s: %v", path, err)
		}
		if err := updateFileRecord(db, storedPath, hash, size, fileTimestamp); err != nil {
			return "", -1, "", fmt.Errorf("failed to update record for file %s: %v", path, err)
		}
		return hash, size, "changed", nil
	}

	return dbHash, dbSize, "existing", nil
}

func getFileMetadata(file *os.File) (int64, time.Time, error) {
	fileInfo, err := file.Stat()
	if err != nil {
		return 0, time.Time{}, err
	}
	return fileInfo.Size(), fileInfo.ModTime(), nil
}

func getDatabaseRecord(db *sql.DB, storedPath string) (string, int64, error) {
	var dbHash string
	var dbSize int64
	err := db.QueryRow("SELECT hash, size FROM file_hashes WHERE filepath = $1", storedPath).Scan(&dbHash, &dbSize)
	return dbHash, dbSize, err
}

func hashFile(file *os.File) (string, error) {
	hasher := md5.New()
	if _, err := file.Seek(0, 0); err != nil {
		return "", err
	}
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

func insertFileRecord(db *sql.DB, storedPath, hash string, size int64, fileTimestamp time.Time) error {
	for {
		_, err := db.Exec("INSERT INTO file_hashes (filepath, hash, size, file_timestamp, hash_calculated_timestamp) VALUES ($1, $2, $3, $4, $5)", storedPath, hash, size, fileTimestamp, time.Now())
		if err == nil {
			return nil
		}
		log.Printf("Retrying INSERT for %s: %v", storedPath, err)
		time.Sleep(1 * time.Second)
	}
}

func updateFileRecord(db *sql.DB, storedPath, hash string, size int64, fileTimestamp time.Time) error {
	for {
		_, err := db.Exec("UPDATE file_hashes SET hash = $1, size = $2, file_timestamp = $3, hash_calculated_timestamp = $4 WHERE filepath = $5", hash, size, fileTimestamp, time.Now(), storedPath)
		if err == nil {
			return nil
		}
		log.Printf("Retrying UPDATE for %s: %v", storedPath, err)
		time.Sleep(1 * time.Second)
	}
}
