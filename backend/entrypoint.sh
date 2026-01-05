#!/bin/bash
set -e

echo "========================================="
echo "Backend Runtime Package Installation"
echo "========================================="

# Check if packages are ACTUALLY installed (not just marker file)
if python -c "import fastapi" 2>/dev/null && \
   python -c "import qdrant_client" 2>/dev/null; then
    echo "✅ Packages already installed, skipping..."
else
    echo "Installing packages (first time or cache corrupted)..."

    # Install OpenTelemetry dependencies
    echo "Installing OpenTelemetry..."
    pip install "opentelemetry-api>=1.25.0" "opentelemetry-sdk>=1.25.0" \
                "opentelemetry-instrumentation-fastapi>=0.46b0" \
                "opentelemetry-instrumentation-requests>=0.46b0"

    # Install main package
    echo "Installing backend package..."
    pip install .

    echo "✅ Package installation complete!"
fi

echo "========================================="
echo "Starting application..."
echo "========================================="

# Run the actual command
exec "$@"
