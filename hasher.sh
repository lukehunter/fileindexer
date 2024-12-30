#!/bin/bash

# Check if sqlite3 and parallel are installed
if ! command -v sqlite3 &> /dev/null || ! command -v parallel &> /dev/null; then
    echo "Error: sqlite3 and/or parallel are not installed. Please install them and try again."
    exit 1
fi

# Check if the required arguments are passed
if [ -z "$1" ] || [ -z "$2" ]; then
    echo "Usage: $0 <target_directory> <database_file> [output_file]

This script calculates the SHA256 hash of all files in a specified directory, stores the hash, file path, and file size in a SQLite database, and generates a report. If a file already exists in the database, it checks if the file size has changed and updates the hash if necessary. The output is saved in a CSV file."
    exit 1
fi

directory="$1"  # Target directory from command line argument
database="$2"   # Database file from command line argument
timestamp=$(date +%Y-%m-%dT%H.%M.%S.%3N)
output_file="${timestamp}_results.csv"  # Default output file

if [ -n "$3" ]; then
    output_file="$3"
fi

# Initialize the output file
echo "filepath,hash,size,status" > "$output_file"

# Create the SQLite database and table if it doesn't exist
sqlite3 "$database" <<EOF
CREATE TABLE IF NOT EXISTS file_hashes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    filepath TEXT NOT NULL,
    hash TEXT NOT NULL,
    size INTEGER NOT NULL,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
);
EOF

# Function to process a single file
process_file() {
    file="$1"
    database="$2"
    output_file="$3"

    # Fetch the existing hash and size from the database
    result=$(sqlite3 "$database" "SELECT hash, size FROM file_hashes WHERE filepath='$file';")

    # Get the file size on disk
    current_size=$(stat --format="%s" "$file")

    if [[ -z "$result" ]]; then
        # No entry in the database, calculate the SHA256 hash
        hash=$(sha256sum "$file" | awk '{print $1}')

        # Insert the hash, file path, and size into the database
        sqlite3 "$database" <<EOF
BEGIN TRANSACTION;
INSERT INTO file_hashes (filepath, hash, size) VALUES ('$file', '$hash', $current_size);
COMMIT;
EOF

        # Write to the output file
        echo "$file,$hash,$current_size,new" >> "$output_file"
    else
        # Parse the database result
        db_hash=$(echo "$result" | awk -F'|' '{print $1}')
        db_size=$(echo "$result" | awk -F'|' '{print $2}')

        if [[ "$current_size" -ne "$db_size" ]]; then
            # File size has changed, mark as "changed"
            hash=$(sha256sum "$file" | awk '{print $1}')
            sqlite3 "$database" <<EOF
BEGIN TRANSACTION;
UPDATE file_hashes SET hash='$hash', size=$current_size WHERE filepath='$file';
COMMIT;
EOF
            echo "$file,$hash,$current_size,changed" >> "$output_file"
        else
            # File size matches, mark as "existing"
            echo "$file,$db_hash,$db_size,existing" >> "$output_file"
        fi
    fi
}

export -f process_file

# Export variables for parallel
export database
export output_file

# Process files in parallel
find "$directory" -type f | parallel process_file {} "$database" "$output_file"

echo "SHA256 hash calculation and storage completed. Results saved to $output_file."

