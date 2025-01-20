#!/bin/bash

# Configuration variables
DB_NAME="files"           # Replace with your database name
DB_USER="luke"            # Replace with your username
DB_HOST="localhost"       # Replace with your host

# Check if a search string is provided
if [ -z "$1" ]; then
  echo "Usage: $0 <search_string>"
  exit 1
fi

SEARCH_STRING=$1

# SQL query to generate the report
SQL_QUERY="
WITH hash_counts AS (
    SELECT hash, COUNT(*) AS copy_count
    FROM file_hashes
    WHERE filepath LIKE '%${SEARCH_STRING}%'
    GROUP BY hash
),
filtered_files AS (
    SELECT hash, copy_count
    FROM hash_counts
    WHERE copy_count >= 1
)
SELECT copy_count, COUNT(*) AS num_files
FROM filtered_files
GROUP BY copy_count
ORDER BY copy_count;
"

# Execute the query using psql
psql -d "$DB_NAME" -U "$DB_USER" -h "$DB_HOST" -c "$SQL_QUERY"
