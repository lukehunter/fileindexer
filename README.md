# File Hashing and Database Storage

This Go application calculates the SHA256 hash of files in a specified directory, stores the hash, file size, and metadata in a PostgreSQL database, and generates a CSV file summarizing the results. It is designed to handle large directories efficiently with parallel processing and robust error handling.

## Features
- Calculates SHA256 hashes for all files in a directory.
- Stores file metadata (path, size, modification time) and hash in a PostgreSQL database.
- Supports prefix removal from file paths when storing in the database.
- Outputs results to a CSV file with details of each file and processing status.
- Handles database insert/update retries for robust operation.
- Parallel file processing with concurrency control.

## Prerequisites
- Go 1.18 or later.
- PostgreSQL database.
- The `github.com/lib/pq` package for PostgreSQL integration.

## Installation
1. Clone the repository:
   ```sh
   git clone <repository_url>
   cd <repository_directory>
   ```
2. Install dependencies:
   ```sh
   go mod tidy
   ```

## Usage
Run the program with the required flags:

```sh
./file-hasher --directory <target_directory> --dbname <postgres_db_name> [--dbuser <user>] [--dbhost <host>] [--dbport <port>] [--prefix <prefix>] [--output <output_file>]
```

### Flags
- `--directory`: The directory to scan for files (required).
- `--dbname`: PostgreSQL database name (required).
- `--dbuser`: PostgreSQL username (default: value of `DB_USER` environment variable).
- `--dbhost`: PostgreSQL host (default: value of `DB_HOST` environment variable).
- `--dbport`: PostgreSQL port (default: value of `DB_PORT` environment variable).
- `--prefix`: A prefix to remove from file paths when storing them in the database (optional).
- `--output`: Path to the output CSV file (default: timestamped filename in the current directory).

### Example
```sh
./file-hasher --directory ./data --dbname filehashdb --dbuser admin --dbhost localhost --dbport 5432 --prefix /data/ --output results.csv
```

## Output
1. **Database**:
   - File metadata and hashes are stored in the PostgreSQL `file_hashes` table.
   - Table schema:
     ```sql
     CREATE TABLE IF NOT EXISTS file_hashes (
         id SERIAL PRIMARY KEY,
         filepath TEXT NOT NULL UNIQUE,
         hash TEXT NOT NULL,
         size INTEGER NOT NULL,
         file_timestamp TIMESTAMP NOT NULL,
         hash_calculated_timestamp TIMESTAMP NOT NULL
     );
     ```

2. **CSV File**:
   - Contains the following columns:
     - `filepath`: File path after removing the specified prefix.
     - `hash`: SHA256 hash of the file.
     - `size`: File size in bytes.
     - `status`: Processing status (`new`, `changed`, `existing`, or error details).

## Error Handling
- Files that cannot be read or processed are logged and recorded in the CSV file with an error message.
- Database operations (`INSERT` and `UPDATE`) include retry logic to handle transient errors.

## Contributing
1. Fork the repository.
2. Create a new branch:
   ```sh
   git checkout -b feature/your-feature
   ```
3. Make your changes and commit them:
   ```sh
   git commit -m "Add your feature"
   ```
4. Push the branch:
   ```sh
   git push origin feature/your-feature
   ```
5. Open a pull request.

## License
This project is licensed under the MIT License. See the `LICENSE` file for details.

