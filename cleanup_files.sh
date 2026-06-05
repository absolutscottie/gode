#!/bin/bash

# Check if a directory was provided
if [ -z "$1" ]; then
    echo "Usage: $0 <directory_path>"
    exit 1
fi

TARGET_DIR="$1"

# Verify the directory exists
if [ ! -d "$TARGET_DIR" ]; then
    echo "Error: Directory '$TARGET_DIR' does not exist."
    exit 1
fi

# Loop through all .go files in the directory
find "$TARGET_DIR" -type f -name "*.toml" | while read -r file; do
    echo "Cleaning: $file"
    
    # Run the tr command using a temporary file
    if tr -cd '\11\12\15\40-\176' < "$file" > "$file.tmp"; then
        mv "$file.tmp" "$file"
    else
        echo "Failed to clean: $file"
        rm -f "$file.tmp"
    fi
done

echo "Done cleaning all Go files."

