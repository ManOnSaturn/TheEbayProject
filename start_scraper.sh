#!/bin/bash

if [ $# -lt 1 ]; then
    echo "You forgot the add a first parameter."
    exit 1
fi

LOCKFILE="/var/lock/scraper.lock"

# Acquire the lock
exec 200>"$LOCKFILE"
flock -n 200 || { echo "Script is already running"; exit 1; }

current_datetime=$(date '+%H-%M')

output_dir="/home/mattia/ebay/logs"

mkdir -p "$output_dir"

output_file="${output_dir}/${current_datetime}.txt"

if [ -f "$output_file" ]; then
    rm "$output_file"
fi

python_script="/home/mattia/ebay/send_error.py"

/home/mattia/ebay/Scraper $1 > "$output_file" 2> >(tee -a "$output_file" | python3 "$python_script")

# Release the lock
flock -u 200
rm -f "$LOCKFILE"