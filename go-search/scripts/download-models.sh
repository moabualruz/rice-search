#!/usr/bin/env bash
# Rice Search - Model Download Script
# Downloads required ONNX models from HuggingFace

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
MODELS_DIR="${MODELS_DIR:-./models}"
HF_CACHE="${HF_HOME:-$HOME/.cache/huggingface}"

# Model definitions
declare -A MODELS=(
    ["jina-embeddings-v3"]="jinaai/jina-embeddings-v3"
    ["splade-v3"]="naver/splade-v3"
    ["jina-reranker-v2"]="jinaai/jina-reranker-v2-base-multilingual"
)

# Functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_dependencies() {
    if ! command -v python3 &> /dev/null; then
        log_error "python3 is required but not installed"
        exit 1
    fi

    # Check for huggingface_hub
    if ! python3 -c "import huggingface_hub" 2>/dev/null; then
        log_warn "huggingface_hub not installed, installing..."
        pip install huggingface-hub
    fi
}

download_model() {
    local name=$1
    local repo=$2
    local target_dir="${MODELS_DIR}/${name}"

    if [[ -d "$target_dir" && -f "$target_dir/model.onnx" ]]; then
        log_info "Model $name already exists, skipping..."
        return 0
    fi

    log_info "Downloading $name from $repo..."
    
    mkdir -p "$target_dir"
    
    # Use huggingface-cli to download
    python3 -c "
from huggingface_hub import snapshot_download
snapshot_download(
    repo_id='${repo}',
    local_dir='${target_dir}',
    local_dir_use_symlinks=False,
    ignore_patterns=['*.bin', '*.safetensors', '*.h5', '*.msgpack']
)
"

    # Check if ONNX model exists
    if [[ ! -f "$target_dir/model.onnx" && ! -f "$target_dir/onnx/model.onnx" ]]; then
        log_warn "ONNX model not found for $name, may need to export from PyTorch"
    fi

    log_info "Downloaded $name successfully"
}

export_to_onnx() {
    local name=$1
    local repo=$2
    local target_dir="${MODELS_DIR}/${name}"

    log_info "Exporting $name to ONNX format..."

    python3 << EOF
from optimum.onnxruntime import ORTModelForFeatureExtraction, ORTModelForSequenceClassification
from transformers import AutoTokenizer
import os

target = "${target_dir}"
repo = "${repo}"
name = "${name}"

try:
    # Different export based on model type
    if "reranker" in name:
        model = ORTModelForSequenceClassification.from_pretrained(
            repo, export=True, provider="CPUExecutionProvider"
        )
    else:
        model = ORTModelForFeatureExtraction.from_pretrained(
            repo, export=True, provider="CPUExecutionProvider"
        )
    
    model.save_pretrained(target)
    
    tokenizer = AutoTokenizer.from_pretrained(repo)
    tokenizer.save_pretrained(target)
    
    print(f"Successfully exported {name} to ONNX")
except Exception as e:
    print(f"Failed to export {name}: {e}")
    exit(1)
EOF
}

main() {
    log_info "Rice Search Model Downloader"
    log_info "Models directory: $MODELS_DIR"
    echo

    check_dependencies

    mkdir -p "$MODELS_DIR"

    for name in "${!MODELS[@]}"; do
        repo="${MODELS[$name]}"
        download_model "$name" "$repo"
        
        # Check if ONNX export is needed
        target_dir="${MODELS_DIR}/${name}"
        if [[ ! -f "$target_dir/model.onnx" ]]; then
            log_warn "ONNX model not found for $name, attempting export..."
            export_to_onnx "$name" "$repo" || log_error "Export failed for $name"
        fi
    done

    echo
    log_info "Download complete!"
    log_info "Models are stored in: $MODELS_DIR"
    echo
    log_info "To verify models, run: rice-search models verify"
}

# Run main
main "$@"
