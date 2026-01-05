#!/bin/bash
set -e

echo "========================================="
echo "Backend Runtime Package Installation"
echo "========================================="

# Check if packages are already installed (marker file)
MARKER="/root/.cache/pip/.packages_installed"

if [ ! -f "$MARKER" ]; then
    echo "Installing packages for the first time..."

    # Install OpenTelemetry dependencies
    echo "Installing OpenTelemetry..."
    pip install "opentelemetry-api>=1.25.0" "opentelemetry-sdk>=1.25.0" \
                "opentelemetry-instrumentation-fastapi>=0.46b0" \
                "opentelemetry-instrumentation-requests>=0.46b0"

    # Install main package
    echo "Installing backend package..."
    pip install .

    # Create marker file
    touch "$MARKER"
    echo "✅ Package installation complete!"
else
    echo "✅ Packages already installed, skipping..."
fi

echo "========================================="
echo "Starting application..."
echo "========================================="

# Run the actual command
exec "$@"
