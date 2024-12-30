#!/bin/bash

# Check if psql is installed
if ! command -v psql &> /dev/null; then
    echo "Error: psql is not installed. Please install it and try again." 
    exit 1
fi

# Check if the required arguments are passed
if [ -z "$1" ] || [ -z "$2" ]; then
    echo "Usage: $0 <target_directory> <postgres_db_name> [output_file]

This script calculates the SHA256 hash of all files in a specified directory, stores the hash, file path, and file size in a PostgreSQL database, and generates a report. If a file already exists in the database, it checks if the file size has changed and updates the hash if necessary. The output is saved in a CSV file." 
    exit 1
fi

directory="$1"  # Target directory from command line argument
db_name="$2"   # PostgreSQL database name from command line argument
timestamp=$(date +%Y-%m-%dT%H.%M.%S.%3N)
output_file="${timestamp}_results.csv"  # Default output file

if [ -n "$3" ]; then
    if [[ "$3" == /* ]] || [[ "$3" == .* ]]; then
        output_file="$3"
    else
        echo "Error: Output file path must be absolute or relative to the current directory." 
        exit 1
    fi
    output_file="$3"
fi

# Initialize the output file
echo "filepath,hash,size,status" > "$output_file"

# Create the PostgreSQL database and table if it doesn't exist
if ! psql "$db_name" <<EOF
CREATE TABLE IF NOT EXISTS file_hashes (
    id SERIAL PRIMARY KEY,
    filepath TEXT NOT NULL UNIQUE,
    hash TEXT NOT NULL,
    size INTEGER NOT NULL,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
EOF
then
    echo "Error: Failed to create table in database $db_name." 
    exit 1
fi

# Function to process a single file
process_file() {
    file="$1"
    db_name="$2"
    output_file="$3"

    # Static progress counter
    echo "Processing: $file"

    # Get the file size on disk
    current_size=$(stat --format="%s" "$file" 2>>error.log || echo -1)

    echo "Current size: $current_size"

    # Calculate the SHA256 hash
    hash=$(sha256sum "$file" | awk '{print $1}')

    # Query the database for existing entry
    result=$(psql "$db_name" -t -c "SELECT hash, size FROM file_hashes WHERE filepath = '$file';" 2>>error.log)
    if [[ $? -ne 0 ]]; then
        echo "Error: Failed to query database for file $file. Exit code: $?"
        return
    fi

    if [[ -z "$result" ]]; then
        # No entry in the database, insert the new record
        if ! psql "$db_name" <<EOF
INSERT INTO file_hashes (filepath, hash, size) VALUES ('$file', '$hash', $current_size);
EOF
        then
            echo "Error: Failed to insert record for file $file. Exit code: $?"
            return
        fi
        echo "$file,$hash,$current_size,new" >> "$output_file"
    else
        # Parse the database result
        db_hash=$(echo "$result" | awk '{print $1}')
        db_size=$(echo "$result" | awk '{print $2}')

        echo "DB size: $db_size"

        if [[ "$current_size" -ne "$db_size" ]]; then
            # File size has changed, update the record
            if ! psql "$db_name" <<EOF
UPDATE file_hashes SET hash = '$hash', size = $current_size WHERE filepath = '$file';
EOF
            then
                echo "Error: Failed to update record for file $file. Exit code: $?"
                return
            fi
            echo "$file,$hash,$current_size,changed" >> "$output_file"
        else
            # File size matches, mark as "existing"
            echo "$file,$db_hash,$db_size,existing" >> "$output_file"
        fi
    fi
}

# Initialize processed file counter
processed_files=0

# Process files sequentially
find "$directory" -type f | while read -r file; do
    process_file "$file" "$db_name" "$output_file"
    ((processed_files++))
    echo "Processed $processed_files files so far."
done

echo "SHA256 hash calculation and storage completed. Results saved to $output_file."

