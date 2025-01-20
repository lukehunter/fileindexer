#!/bin/bash

# Input variables
DB_NAME="files"       # Name of the database
TABLE_NAME="file_hashes"

psql -d "$DB_NAME" -c "SELECT pg_size_pretty(pg_database_size('$DB_NAME'))" -h 192.168.1.63 -U luke

psql -d "$DB_NAME" -c "select count(*) from $TABLE_NAME;" -h 192.168.1.63 -U luke
