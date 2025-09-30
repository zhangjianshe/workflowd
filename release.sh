#!/bin/bash

# --- Automated Workflow Release Script ---
# This script reads the version from version.txt, increments the patch number,
# commits the change, tags the commit, and executes the 'make release' target.

set -e # Exit immediately if a command exits with a non-zero status

VERSION_FILE="version.txt"

if [ ! -f "$VERSION_FILE" ]; then
    echo "Error: Version file '$VERSION_FILE' not found."
    exit 1
fi

# 1. Read current version (e.g., v1.0.1)
CURRENT_VERSION=$(cat "$VERSION_FILE")
echo "Current version: $CURRENT_VERSION"

# 2. Extract and increment the patch number
# Use awk to split by dot, capture the prefix (vX.Y), and increment the last part
MAJOR_MINOR=$(echo "$CURRENT_VERSION" | awk -F'.' '{print $1"."$2}')
PATCH=$(echo "$CURRENT_VERSION" | awk -F'.' '{print $3}')
NEXT_PATCH=$((PATCH + 1))

# 3. Assemble the new version string (e.g., v1.0.2)
NEW_VERSION="${MAJOR_MINOR}.${NEXT_PATCH}"
echo "New version: $NEW_VERSION"

# 4. Update the version file
echo "$NEW_VERSION" > "$VERSION_FILE"

# 5. Git Commit the version change
git add "$VERSION_FILE"
git commit -m "Release: $NEW_VERSION - Incremented patch version."

# 6. Git Tag the new release
git tag -a "$NEW_VERSION" -m "Release version $NEW_VERSION"
echo "Created Git tag: $NEW_VERSION"

# 7. Execute the 'make release' build target
echo "Starting release build..."
make release

echo "--- Release $NEW_VERSION Process Complete! ---"
echo "Binaries are in the 'dist' folder, and Git tag is created."
