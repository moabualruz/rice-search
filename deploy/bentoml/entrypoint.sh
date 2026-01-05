#!/bin/bash
set -e

echo "========================================="
echo "BentoML Runtime Package Installation"
echo "========================================="

# Check if packages are ACTUALLY installed (not just marker file)
if python3.12 -c "import bentoml" 2>/dev/null && \
   python3.12 -c "import torch" 2>/dev/null && \
   python3.12 -c "import sglang" 2>/dev/null; then
    echo "✅ Packages already installed, skipping..."
else
    echo "Installing packages (first time or cache corrupted)..."

    # Install SGLang first - it will install its own compatible versions
    # Let SGLang dictate dependency versions (transformers, torch, etc.)
    echo "Installing all packages (pip will resolve versions)..."
    pip3.12 install \
        "sglang[all]==0.5.7" \
        bentoml==1.3.15 \
        accelerate \
        pydantic \
        einops \
        httpx

    echo "✅ Package installation complete!"
fi

echo "========================================="
echo "Starting BentoML service..."
echo "========================================="

# Suppress pkg_resources deprecation warning from fs library
export PYTHONWARNINGS="ignore::UserWarning:fs"

# Run the actual command
exec "$@"
