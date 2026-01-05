#!/bin/bash
set -e

echo "========================================="
echo "Unified Inference - Package Installation"
echo "========================================="

# Check if packages are installed
if python3.12 -c "import sglang" 2>/dev/null && \
   python3.12 -c "import flask" 2>/dev/null && \
   python3.12 -c "import httpx" 2>/dev/null; then
    echo "✅ Packages already installed, skipping..."
else
    echo "Installing packages..."
    pip3.12 install --ignore-installed \
        "sglang[all]==0.5.7" \
        flask \
        httpx
    echo "✅ Packages installed!"
fi

echo "========================================="
echo "Starting Unified Inference Service..."
echo "========================================="

exec python3.12 /app/service.py
