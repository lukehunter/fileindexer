# File Hashing and Database Storage

## Overview
This Go application calculates the hashes of files in the specified directory, stores the hash, file size, and
other metadata in a PostgreSQL database, and generates a CSV file summarizing the results. 

## Why this exists
If you have a bunch of hard drives to manage, some of them possibly offline, and you want to track where files 
exist, how many copies there are, etc. Helps to identify any files that may not be properly backed up on a backup 
drive, or identify when there are extra copies (either more than a primary and replica drive, or in more than one 
location on a drive) that can be deleted. Note that this tool relies on files having unique paths on the different 
drives they're stored in (file path is UNIQUE in the database schema).

## Example Usage
In this case, I have mounted some external drives to e.g. /mnt/i and /mnt/h. The two drives have unique root folder 
names in order to disambiguate any files that may be backups of each other. The output of the command will include 
any new, existing, or changed files that were found, and the database will be updated with hashes of any newly 
scanned files. Hashes are re-calculated only if the file size recorded in the database does not match. If you are 
not expecting the indexed files to change (e.g. in the case of original photo or video archives) and are intent on 
monitoring for bit-rot, make sure to hold on to / review the csv output for rows with "changed" in them.

```sh
./fileindexer --directory /mnt/i --dbname files --dbuser <dbuser> --dbhost <host> --dbport <port> --prefix /mnt/i 
--exclude .bzvol,$RECYCLE.BIN
./fileindexer --directory /mnt/h --dbname files --dbuser <dbuser> --dbhost <host> --dbport <port> --prefix /mnt/h 
--exclude .bzvol,$RECYCLE.BIN
```

## Features
- Calculates SHA256 hashes for all files in a directory. 
- Stores file metadata (path, size, modification time) and hash in a PostgreSQL database.
- Supports prefix removal from file paths when storing in the database.
- Outputs results to a CSV file with details of each file and processing status.
- Handles database insert/update retries for robust operation.
- Parallel file processing with concurrency control.

## TODO
- missing file handling
- re-hashing files which haven't been hashed in specified time window
- code cleanup

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

## Output
1. **Database**:
   - File metadata and hashes are stored in the PostgreSQL `file_hashes` table.

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

