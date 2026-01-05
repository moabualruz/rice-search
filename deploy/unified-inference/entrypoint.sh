#!/bin/bash
set -e

echo "========================================="
echo "Unified-Inference Runtime Setup"
echo "========================================="

# Check if packages are installed
if python -c "import fastapi" 2>/dev/null && \
   python -c "import sglang" 2>/dev/null; then
    echo "✅ Packages already installed, skipping..."
else
    echo "Installing packages (first time or cache corrupted)..."
    
    # Install the package
    pip install .
    
    echo "✅ Package installation complete!"
fi

echo "========================================="
echo "Starting unified-inference service..."
echo "========================================="

# Run the actual command
exec "$@"
