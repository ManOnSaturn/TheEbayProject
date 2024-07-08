#!/bin/bash

current_datetime=$(date '+%H-%M')

output_dir="/home/pi/ebay/logs"

mkdir -p "$output_dir"

output_file="${output_dir}/${current_datetime}.txt"

if [ -f "$output_file" ]; then
    rm "$output_file"
fi

python_script="/home/pi/ebay/send_error.py"

/home/pi/ebay/Scraper > "$output_file" 2> >(tee -a "$output_file" | python3 "$python_script")