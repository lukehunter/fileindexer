#!/bin/bash

# Input variables
DB_NAME="files"       # Name of the database
TABLE_NAME="file_hashes"

# Confirmation prompt
read -p "Are you sure you want to drop the table '${TABLE_NAME}' from database '${DB_NAME}'? This action cannot be undone. (yes/no): " CONFIRMATION

# Check user confirmation
if [[ "$CONFIRMATION" != "yes" ]]; then
    echo "Operation canceled. The table '${TABLE_NAME}' was not dropped."
    exit 0
fi

# Command to drop the table
echo "Dropping table '${TABLE_NAME}' from database '${DB_NAME}'..."
psql -d "$DB_NAME" -c "DROP TABLE IF EXISTS $TABLE_NAME;"

# Check if the drop command was successful
if [ $? -eq 0 ]; then
    echo "Table '${TABLE_NAME}' successfully dropped from database '${DB_NAME}'."
else
    echo "Failed to drop table '${TABLE_NAME}' from database '${DB_NAME}'."
fi
