#!/bin/bash

current_datetime=$(date '+%H-%M')

output_dir="/home/mattia/ebay/logs"

mkdir -p "$output_dir"

output_file="${output_dir}/${current_datetime}.txt"

if [ -f "$output_file" ]; then
    rm "$output_file"
fi

python_script="/home/mattia/ebay/send_error.py"

/home/mattia/ebay/Scraper > "$output_file" 2> >(tee -a "$output_file" | python3 "$python_script")