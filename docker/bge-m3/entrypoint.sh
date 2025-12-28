#!/bin/bash
set -e

echo "=== BGE-M3 Container Startup ==="

# Install dependencies (uses mounted pip cache at /root/.cache/pip)
echo "Installing dependencies..."
pip install -r /app/requirements.txt

echo "Starting server..."
exec uvicorn main:app --host 0.0.0.0 --port 80
