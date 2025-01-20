#!/bin/bash

# Configuration variables
DB_NAME="files"           # Replace with your database name
DB_USER="luke"            # Replace with your username
DB_HOST="localhost"       # Replace with your host

# Check if two strings are provided
if [ -z "$1" ] || [ -z "$2" ]; then
  echo "Usage: $0 <search_string_1> <search_string_2>"
  exit 1
fi

SEARCH_STRING_1=$1
SEARCH_STRING_2=$2

# SQL query
SQL_QUERY="
WITH files_1 AS (
    SELECT hash, filepath
    FROM file_hashes
    WHERE filepath LIKE '%${SEARCH_STRING_1}%'
),
files_2 AS (
    SELECT hash, filepath
    FROM file_hashes
    WHERE filepath LIKE '%${SEARCH_STRING_2}%'
)
SELECT f1.filepath AS file_1_path, 
       f1.hash AS file_1_hash, 
       f2.filepath AS matching_file_2_path
FROM files_1 f1
LEFT JOIN files_2 f2
ON f1.hash = f2.hash
WHERE f2.filepath IS NULL
ORDER BY f1.filepath;
"

# Execute the query using psql
psql -d "$DB_NAME" -U "$DB_USER" -h "$DB_HOST" -c "$SQL_QUERY"
