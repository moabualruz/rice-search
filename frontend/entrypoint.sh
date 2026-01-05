#!/bin/sh
set -e

echo "========================================="
echo "Frontend Runtime Package Installation"
echo "========================================="

# Check if node_modules exists and has packages
MARKER="/root/.npm/.packages_installed"

if [ ! -f "$MARKER" ]; then
    echo "Installing npm packages for the first time..."
    npm install

    # Create marker file
    touch "$MARKER"
    echo "✅ npm install complete!"
else
    echo "✅ Packages already installed, skipping..."
fi

echo "========================================="
echo "Starting Next.js dev server..."
echo "========================================="

# Run the actual command
exec "$@"
