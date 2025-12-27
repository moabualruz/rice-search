#!/bin/bash
#
# Download Tree-sitter WASM files from jsDelivr CDN
# Used during Docker image build - overrides any existing files
#
# Usage: ./download-wasm.sh [output_dir] [--force]
#

set -e

WASM_DIR="${1:-/app/wasm}"
FORCE="${2:-}"

# Use latest version from CDN
CDN_URL="https://cdn.jsdelivr.net/npm/tree-sitter-wasms/out"

# All available languages in tree-sitter-wasms@0.1.13
LANGUAGES=(
  # Web / Frontend
  "tree-sitter-javascript"
  "tree-sitter-typescript"
  "tree-sitter-tsx"
  "tree-sitter-html"
  "tree-sitter-css"
  "tree-sitter-vue"
  
  # Systems / Backend
  "tree-sitter-python"
  "tree-sitter-rust"
  "tree-sitter-go"
  "tree-sitter-java"
  "tree-sitter-kotlin"
  "tree-sitter-scala"
  "tree-sitter-c"
  "tree-sitter-cpp"
  "tree-sitter-c_sharp"
  "tree-sitter-swift"
  "tree-sitter-objc"
  "tree-sitter-dart"
  
  # Scripting
  "tree-sitter-ruby"
  "tree-sitter-php"
  "tree-sitter-lua"
  "tree-sitter-elixir"
  "tree-sitter-ocaml"
  "tree-sitter-elm"
  "tree-sitter-rescript"
  "tree-sitter-elisp"
  
  # Shell / Config
  "tree-sitter-bash"
  
  # Data / Config formats
  "tree-sitter-json"
  "tree-sitter-yaml"
  "tree-sitter-toml"
  
  # Other
  "tree-sitter-zig"
  "tree-sitter-solidity"
  "tree-sitter-ql"
  "tree-sitter-tlaplus"
  "tree-sitter-systemrdl"
  "tree-sitter-embedded_template"
)

echo "========================================"
echo "Tree-sitter WASM Downloader"
echo "========================================"
echo "CDN: ${CDN_URL}"
echo "Output: ${WASM_DIR}"
echo "Languages: ${#LANGUAGES[@]}"
if [ "${FORCE}" = "--force" ]; then
  echo "Mode: FORCE (overwriting existing files)"
fi
echo ""

# Create output directory
mkdir -p "${WASM_DIR}"

SUCCESS=0
FAILED=0
SKIPPED=0

for LANG in "${LANGUAGES[@]}"; do
  WASM_FILE="${WASM_DIR}/${LANG}.wasm"
  
  # Skip if exists and not forcing
  if [ -f "${WASM_FILE}" ] && [ "${FORCE}" != "--force" ]; then
    echo "✓ ${LANG}.wasm (exists)"
    SKIPPED=$((SKIPPED + 1))
    continue
  fi
  
  # Download (force overwrites existing)
  echo -n "⬇ ${LANG}.wasm... "
  if curl -sSfL "${CDN_URL}/${LANG}.wasm" -o "${WASM_FILE}" 2>/dev/null; then
    SIZE=$(du -h "${WASM_FILE}" | cut -f1)
    echo "✓ (${SIZE})"
    SUCCESS=$((SUCCESS + 1))
  else
    echo "✗ failed"
    rm -f "${WASM_FILE}"
    FAILED=$((FAILED + 1))
  fi
done

echo ""
echo "========================================"
echo "Results: ${SUCCESS} downloaded, ${SKIPPED} skipped, ${FAILED} failed"
echo "========================================"

# Exit with error if any failed
if [ ${FAILED} -gt 0 ]; then
  exit 1
fi
