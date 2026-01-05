#!/bin/bash
set -e

echo "========================================="
echo "BentoML Runtime Package Installation"
echo "========================================="

# Check if packages are already installed (marker file)
MARKER="/root/.cache/pip/.packages_installed"

if [ ! -f "$MARKER" ]; then
    echo "Installing packages for the first time..."

    # Install PyTorch 2.5.1 with CUDA 12.1
    echo "Installing PyTorch..."
    pip3.12 install torch==2.5.1 --index-url https://download.pytorch.org/whl/cu121

    # Install ML dependencies
    echo "Installing ML dependencies..."
    pip3.12 install \
        bentoml==1.3.15 \
        transformers==4.57.3 \
        accelerate==1.12.0 \
        pydantic==2.12.5 \
        einops \
        httpx

    # Install SGLang
    echo "Installing SGLang..."
    pip3.12 install "sglang[all]==0.5.7"

    # Create marker file
    touch "$MARKER"
    echo "✅ Package installation complete!"
else
    echo "✅ Packages already installed, skipping..."
fi

echo "========================================="
echo "Starting BentoML service..."
echo "========================================="

# Run the actual command
exec "$@"
