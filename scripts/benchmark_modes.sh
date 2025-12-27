#!/bin/bash
# =============================================================================
# Rice Search Benchmark Script
# Compares mixedbread vs bge-m3 modes on CPU and GPU
#
# This script:
# 1. Uses isolated data folder (./benchmark_data/) - not default ./data/
# 2. Runs CPU benchmarks for both modes
# 3. Runs GPU benchmarks for both modes (if GPU available)
# 4. Generates comparison report
#
# Usage:
#   bash scripts/benchmark_modes.sh                    # Full benchmark
#   bash scripts/benchmark_modes.sh --cpu-only         # CPU only
#   bash scripts/benchmark_modes.sh --gpu-only         # GPU only
#   bash scripts/benchmark_modes.sh --path ./api       # Custom index path
# =============================================================================

set -e

# ============================================
# Configuration
# ============================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BENCHMARK_DATA_DIR="$PROJECT_DIR/benchmark_data"
RESULTS_DIR="$PROJECT_DIR/benchmark_results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_FILE="$RESULTS_DIR/benchmark_report_${TIMESTAMP}.md"
INDEX_PATH="$PROJECT_DIR"
API_PORT=8085  # Use different port to avoid conflicts with running services

# Timeouts
STARTUP_TIMEOUT=300  # 5 minutes for services to start
INDEX_TIMEOUT=600    # 10 minutes for indexing
SEARCH_TIMEOUT=60    # 1 minute per search

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# Default queries for benchmarking
QUERIES=(
  "search service implementation"
  "embedding service vector"
  "milvus database connection"
  "tantivy bm25 sparse search"
  "hybrid ranking rrf fusion"
  "reranker neural model"
  "query classifier detection"
  "tree sitter chunker parser"
  "nestjs controller endpoint api"
  "async await promise handling"
)

# Parse arguments
CPU_ONLY=false
GPU_ONLY=false
SKIP_INDEX=false

while [[ $# -gt 0 ]]; do
  case $1 in
    --cpu-only)
      CPU_ONLY=true
      shift
      ;;
    --gpu-only)
      GPU_ONLY=true
      shift
      ;;
    --path)
      INDEX_PATH="$2"
      shift 2
      ;;
    --skip-index)
      SKIP_INDEX=true
      shift
      ;;
    --help|-h)
      echo "Usage: $0 [OPTIONS]"
      echo ""
      echo "Options:"
      echo "  --cpu-only      Run CPU benchmarks only"
      echo "  --gpu-only      Run GPU benchmarks only"
      echo "  --path DIR      Directory to index (default: project root)"
      echo "  --skip-index    Skip indexing, use existing data"
      echo ""
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

# ============================================
# Helper Functions
# ============================================
log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_pass() { echo -e "${GREEN}[PASS]${NC} $1"; }
log_fail() { echo -e "${RED}[FAIL]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_header() { 
  echo ""
  echo -e "${CYAN}════════════════════════════════════════════════════════════${NC}"
  echo -e "${CYAN}${BOLD}  $1${NC}"
  echo -e "${CYAN}════════════════════════════════════════════════════════════${NC}"
}

check_prerequisites() {
  log_info "Checking prerequisites..."
  
  # Check docker
  if ! command -v docker &> /dev/null; then
    log_fail "Docker is required but not installed"
    exit 1
  fi
  
  # Check docker compose
  if ! docker compose version &> /dev/null; then
    log_fail "Docker Compose V2 is required"
    exit 1
  fi
  
  # Check jq
  if ! command -v jq &> /dev/null; then
    log_fail "jq is required but not installed"
    exit 1
  fi
  
  # Check curl
  if ! command -v curl &> /dev/null; then
    log_fail "curl is required but not installed"
    exit 1
  fi
  
  # Check python3
  if ! command -v python3 &> /dev/null; then
    log_fail "python3 is required but not installed"
    exit 1
  fi
  
  log_pass "All prerequisites met"
}

check_gpu_available() {
  if command -v nvidia-smi &> /dev/null && nvidia-smi &> /dev/null; then
    return 0
  else
    return 1
  fi
}

# ============================================
# Docker Compose Management
# ============================================
create_benchmark_compose() {
  local gpu_mode=$1
  local compose_file="$BENCHMARK_DATA_DIR/docker-compose.benchmark.yml"
  
  mkdir -p "$BENCHMARK_DATA_DIR"
  
  cat > "$compose_file" << 'COMPOSE_EOF'
# Benchmark-specific docker-compose override
# Uses isolated data folder for benchmarking

services:
  etcd:
    volumes:
      - ./etcd:/etcd

  minio:
    volumes:
      - ./minio:/minio_data

  milvus:
    volumes:
      - ./milvus:/var/lib/milvus

  infinity:
    volumes:
      - ./infinity-cache:/app/.cache

  bge-m3:
    volumes:
      - ./bge-m3-cache:/app/.cache

  api:
    ports:
      - "8085:8080"
    volumes:
      - ./api:/data
      - ./tantivy:/tantivy
COMPOSE_EOF
  
  echo "$compose_file"
}

start_services() {
  local mode=$1      # "cpu" or "gpu"
  local embed_mode=$2  # "mixedbread" or "bge-m3"
  
  log_info "Starting services in $mode mode for $embed_mode..."
  
  cd "$BENCHMARK_DATA_DIR"
  
  # Build compose command
  local compose_cmd="docker compose -f $PROJECT_DIR/docker-compose.yml -f docker-compose.benchmark.yml"
  
  if [ "$mode" = "gpu" ]; then
    compose_cmd="$compose_cmd -f $PROJECT_DIR/docker-compose.gpu.yml"
  fi
  
  # Add bge-m3 profile if needed
  if [ "$embed_mode" = "bge-m3" ]; then
    compose_cmd="$compose_cmd --profile bge-m3"
  fi
  
  # Stop any existing containers
  $compose_cmd down --remove-orphans 2>/dev/null || true
  
  # Start services
  log_info "Running: $compose_cmd up -d"
  $compose_cmd up -d
  
  # Wait for services to be healthy
  wait_for_services "$embed_mode"
  
  cd "$PROJECT_DIR"
}

stop_services() {
  log_info "Stopping benchmark services..."
  
  if [ -d "$BENCHMARK_DATA_DIR" ]; then
    cd "$BENCHMARK_DATA_DIR"
    
    # Try to stop with all possible profiles
    docker compose -f "$PROJECT_DIR/docker-compose.yml" -f docker-compose.benchmark.yml --profile bge-m3 down --remove-orphans 2>/dev/null || true
    
    cd "$PROJECT_DIR"
  fi
}

wait_for_services() {
  local embed_mode=$1
  local api_url="http://localhost:$API_PORT"
  local waited=0
  
  log_info "Waiting for API to be ready..."
  
  while [ $waited -lt $STARTUP_TIMEOUT ]; do
    if curl -s -o /dev/null -w "%{http_code}" --max-time 5 "$api_url/healthz" 2>/dev/null | grep -q "200"; then
      log_pass "API is ready"
      
      # If bge-m3 mode, also wait for bge-m3 service
      if [ "$embed_mode" = "bge-m3" ]; then
        log_info "Waiting for BGE-M3 service..."
        local bge_waited=0
        while [ $bge_waited -lt $STARTUP_TIMEOUT ]; do
          if curl -s -o /dev/null -w "%{http_code}" --max-time 5 "http://localhost:8083/health" 2>/dev/null | grep -q "200"; then
            log_pass "BGE-M3 service is ready"
            return 0
          fi
          sleep 5
          bge_waited=$((bge_waited + 5))
          echo -n "."
        done
        echo ""
        log_fail "BGE-M3 service not ready after ${STARTUP_TIMEOUT}s"
        return 1
      fi
      
      return 0
    fi
    sleep 5
    waited=$((waited + 5))
    echo -n "."
  done
  
  echo ""
  log_fail "API not ready after ${STARTUP_TIMEOUT}s"
  return 1
}

cleanup_data() {
  log_info "Cleaning up benchmark data..."
  
  # Stop services first
  stop_services
  
  # Remove data directories (keep compose file and caches)
  if [ -d "$BENCHMARK_DATA_DIR" ]; then
    rm -rf "$BENCHMARK_DATA_DIR/etcd" 2>/dev/null || true
    rm -rf "$BENCHMARK_DATA_DIR/minio" 2>/dev/null || true
    rm -rf "$BENCHMARK_DATA_DIR/milvus" 2>/dev/null || true
    rm -rf "$BENCHMARK_DATA_DIR/api" 2>/dev/null || true
    rm -rf "$BENCHMARK_DATA_DIR/tantivy" 2>/dev/null || true
    # Keep cache folders for faster re-runs
    # rm -rf "$BENCHMARK_DATA_DIR/infinity-cache" 2>/dev/null || true
    # rm -rf "$BENCHMARK_DATA_DIR/bge-m3-cache" 2>/dev/null || true
  fi
  
  log_pass "Benchmark data cleaned"
}

# ============================================
# Indexing
# ============================================
index_files() {
  local embed_mode=$1
  local api_url="http://localhost:$API_PORT"
  
  log_info "Indexing files with mode=$embed_mode from $INDEX_PATH..."
  
  # Use the reindex.py script with mode parameter
  python3 "$PROJECT_DIR/scripts/reindex.py" "$INDEX_PATH" \
    --api-url "$api_url" \
    --store "benchmark" \
    --mode "$embed_mode" \
    --force
  
  log_pass "Indexing complete"
}

# ============================================
# Search Benchmarking
# ============================================
run_searches() {
  local embed_mode=$1
  local hw_mode=$2  # "cpu" or "gpu"
  local api_url="http://localhost:$API_PORT"
  local output_file="$RESULTS_DIR/${TIMESTAMP}_${hw_mode}_${embed_mode}.jsonl"
  
  log_info "Running search benchmark: $hw_mode + $embed_mode"
  
  mkdir -p "$RESULTS_DIR"
  
  local total_latency=0
  local count=0
  
  for query in "${QUERIES[@]}"; do
    local start_time=$(date +%s%3N 2>/dev/null || python3 -c 'import time; print(int(time.time()*1000))')
    
    local response=$(curl -s --max-time $SEARCH_TIMEOUT -X POST "$api_url/v1/stores/benchmark/search" \
      -H "Content-Type: application/json" \
      -d "{
        \"query\": \"$query\",
        \"top_k\": 10,
        \"mode\": \"$embed_mode\",
        \"include_content\": false
      }" 2>/dev/null)
    
    local end_time=$(date +%s%3N 2>/dev/null || python3 -c 'import time; print(int(time.time()*1000))')
    local latency=$((end_time - start_time))
    
    # Add metadata to response
    local result=$(echo "$response" | jq -c \
      --arg latency "$latency" \
      --arg query "$query" \
      --arg mode "$embed_mode" \
      --arg hw "$hw_mode" \
      '. + {latency_ms: ($latency | tonumber), query: $query, mode: $mode, hardware: $hw}' 2>/dev/null || \
      echo "{\"error\": \"parse_failed\", \"latency_ms\": $latency, \"query\": \"$query\", \"mode\": \"$embed_mode\", \"hardware\": \"$hw_mode\"}")
    
    echo "$result" >> "$output_file"
    
    local result_count=$(echo "$response" | jq -r '.total // 0' 2>/dev/null || echo "0")
    total_latency=$((total_latency + latency))
    count=$((count + 1))
    
    printf "  %-45s %4dms  %2d results\n" "\"${query:0:42}\"" "$latency" "$result_count"
  done
  
  local avg_latency=$((total_latency / count))
  echo ""
  log_pass "$hw_mode + $embed_mode: avg ${avg_latency}ms over $count queries"
  
  # Return average latency
  echo "$avg_latency"
}

# ============================================
# Report Generation
# ============================================
generate_report() {
  log_header "Generating Report"
  
  mkdir -p "$RESULTS_DIR"
  
  cat > "$REPORT_FILE" << REPORT_HEADER
# Rice Search Benchmark Report

**Generated:** $(date)
**Index Path:** $INDEX_PATH
**Queries:** ${#QUERIES[@]}

## Architecture

| Mode | Dense Embeddings | Sparse Search | Fusion |
|------|------------------|---------------|--------|
| mixedbread | Infinity (mxbai-embed-large-v1) → Milvus | Tantivy BM25 | RRF (app layer) |
| bge-m3 | BGE-M3 → Milvus hybrid | BGE-M3 sparse → Milvus | RRF (Milvus native) |

## Results Summary

REPORT_HEADER

  # Calculate averages for each configuration
  echo "| Configuration | Avg Latency (ms) | Min | Max | Queries |" >> "$REPORT_FILE"
  echo "|---------------|------------------|-----|-----|---------|" >> "$REPORT_FILE"
  
  for file in "$RESULTS_DIR"/${TIMESTAMP}_*.jsonl; do
    if [ -f "$file" ]; then
      local basename=$(basename "$file" .jsonl)
      local config="${basename#${TIMESTAMP}_}"
      
      local stats=$(jq -s '
        {
          avg: ([.[].latency_ms] | add / length | floor),
          min: ([.[].latency_ms] | min),
          max: ([.[].latency_ms] | max),
          count: length
        }
      ' "$file" 2>/dev/null)
      
      local avg=$(echo "$stats" | jq -r '.avg')
      local min=$(echo "$stats" | jq -r '.min')
      local max=$(echo "$stats" | jq -r '.max')
      local count=$(echo "$stats" | jq -r '.count')
      
      echo "| $config | $avg | $min | $max | $count |" >> "$REPORT_FILE"
    fi
  done
  
  # Add detailed results
  cat >> "$REPORT_FILE" << 'REPORT_DETAILS'

## Detailed Results

### Per-Query Latencies

REPORT_DETAILS

  # Create comparison table
  echo "| Query | CPU+mixedbread | CPU+bge-m3 | GPU+mixedbread | GPU+bge-m3 |" >> "$REPORT_FILE"
  echo "|-------|----------------|------------|----------------|------------|" >> "$REPORT_FILE"
  
  for query in "${QUERIES[@]}"; do
    local short_query="${query:0:30}"
    local cpu_mb=$(jq -r "select(.query == \"$query\") | .latency_ms" "$RESULTS_DIR/${TIMESTAMP}_cpu_mixedbread.jsonl" 2>/dev/null || echo "-")
    local cpu_bg=$(jq -r "select(.query == \"$query\") | .latency_ms" "$RESULTS_DIR/${TIMESTAMP}_cpu_bge-m3.jsonl" 2>/dev/null || echo "-")
    local gpu_mb=$(jq -r "select(.query == \"$query\") | .latency_ms" "$RESULTS_DIR/${TIMESTAMP}_gpu_mixedbread.jsonl" 2>/dev/null || echo "-")
    local gpu_bg=$(jq -r "select(.query == \"$query\") | .latency_ms" "$RESULTS_DIR/${TIMESTAMP}_gpu_bge-m3.jsonl" 2>/dev/null || echo "-")
    
    echo "| $short_query | ${cpu_mb}ms | ${cpu_bg}ms | ${gpu_mb}ms | ${gpu_bg}ms |" >> "$REPORT_FILE"
  done
  
  cat >> "$REPORT_FILE" << REPORT_FOOTER

## Files

Raw results are in: \`$RESULTS_DIR/\`

- \`${TIMESTAMP}_cpu_mixedbread.jsonl\`
- \`${TIMESTAMP}_cpu_bge-m3.jsonl\`
- \`${TIMESTAMP}_gpu_mixedbread.jsonl\`
- \`${TIMESTAMP}_gpu_bge-m3.jsonl\`
REPORT_FOOTER

  log_pass "Report generated: $REPORT_FILE"
}

# ============================================
# Main Benchmark Flow
# ============================================
run_benchmark_for_mode() {
  local hw_mode=$1      # "cpu" or "gpu"
  local embed_mode=$2   # "mixedbread" or "bge-m3"
  
  log_header "Benchmark: $hw_mode + $embed_mode"
  
  # Clean up previous data
  cleanup_data
  
  # Create compose file
  create_benchmark_compose "$hw_mode"
  
  # Start services
  if ! start_services "$hw_mode" "$embed_mode"; then
    log_fail "Failed to start services for $hw_mode + $embed_mode"
    stop_services
    return 1
  fi
  
  # Index files
  if ! index_files "$embed_mode"; then
    log_fail "Failed to index for $hw_mode + $embed_mode"
    stop_services
    return 1
  fi
  
  # Wait for index to be ready
  sleep 5
  
  # Run searches
  run_searches "$embed_mode" "$hw_mode"
  
  # Stop services
  stop_services
  
  log_pass "Completed: $hw_mode + $embed_mode"
}

main() {
  log_header "Rice Search Benchmark"
  echo ""
  echo "  Comparing: mixedbread vs bge-m3"
  echo "  Hardware:  CPU and GPU"
  echo "  Index:     $INDEX_PATH"
  echo ""
  
  # Prerequisites
  check_prerequisites
  
  # Check GPU availability
  local has_gpu=false
  if check_gpu_available; then
    log_pass "GPU detected"
    has_gpu=true
  else
    log_warn "No GPU detected - will skip GPU benchmarks"
  fi
  
  # Create results directory
  mkdir -p "$RESULTS_DIR"
  
  # ========== CPU Benchmarks ==========
  if [ "$GPU_ONLY" = false ]; then
    log_header "CPU Benchmarks"
    
    # CPU + mixedbread
    run_benchmark_for_mode "cpu" "mixedbread"
    
    # CPU + bge-m3
    run_benchmark_for_mode "cpu" "bge-m3"
  fi
  
  # ========== GPU Benchmarks ==========
  if [ "$CPU_ONLY" = false ] && [ "$has_gpu" = true ]; then
    log_header "GPU Benchmarks"
    
    # GPU + mixedbread
    run_benchmark_for_mode "gpu" "mixedbread"
    
    # GPU + bge-m3
    run_benchmark_for_mode "gpu" "bge-m3"
  elif [ "$CPU_ONLY" = false ] && [ "$has_gpu" = false ]; then
    log_warn "Skipping GPU benchmarks - no GPU available"
  fi
  
  # ========== Final Cleanup ==========
  log_header "Cleanup"
  cleanup_data
  rm -rf "$BENCHMARK_DATA_DIR" 2>/dev/null || true
  log_pass "Benchmark data cleaned up"
  
  # ========== Generate Report ==========
  generate_report
  
  # ========== Summary ==========
  log_header "Benchmark Complete!"
  echo ""
  echo -e "  ${BOLD}Report:${NC} $REPORT_FILE"
  echo -e "  ${BOLD}Raw Data:${NC} $RESULTS_DIR/"
  echo ""
  echo "  View report with:"
  echo "    cat $REPORT_FILE"
  echo ""
}

# Trap to cleanup on exit
cleanup_on_exit() {
  log_warn "Interrupted - cleaning up..."
  stop_services
  exit 1
}
trap cleanup_on_exit INT TERM

main "$@"
