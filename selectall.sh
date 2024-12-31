#!/bin/bash

# Input variables
DB_NAME="files"       # Name of the database
TABLE_NAME="file_hashes"


psql -d "$DB_NAME" -c "select * from file_hashes;"

# Check if the drop command was successful
if [ $? -eq 0 ]; then
    echo "Success."
else
    echo "Failed."
fi