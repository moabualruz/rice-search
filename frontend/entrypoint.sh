#!/bin/sh
set -e

echo "========================================="
echo "Frontend Runtime Package Installation"
echo "========================================="

# Check if node_modules exists and has critical packages
if [ -d "node_modules" ] && [ -d "node_modules/next" ] && [ -d "node_modules/react" ]; then
    echo "✅ Packages already installed, skipping..."
else
    echo "Installing npm packages (first time or cache corrupted)..."
    npm install
    echo "✅ npm install complete!"
fi

echo "========================================="
echo "Starting Next.js dev server..."
echo "========================================="

# Run the actual command
exec "$@"
