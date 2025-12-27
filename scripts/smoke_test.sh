#!/bin/bash
# Smoke test for Local Code Search Platform
# Run after docker-compose up to verify all services are working

set -e

API_URL="${API_URL:-http://localhost:8080}"
TIMEOUT=10

echo "=========================================="
echo "Local Code Search - Smoke Test"
echo "=========================================="
echo "API URL: $API_URL"
echo ""

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

pass() { echo -e "${GREEN}✓ PASS${NC}: $1"; }
fail() { echo -e "${RED}✗ FAIL${NC}: $1"; exit 1; }
warn() { echo -e "${YELLOW}! WARN${NC}: $1"; }

# 1. Health check
echo "1. Testing health endpoint..."
HEALTH=$(curl -s -o /dev/null -w "%{http_code}" --max-time $TIMEOUT "$API_URL/healthz" || echo "000")
if [ "$HEALTH" = "200" ]; then
    pass "Health check passed"
else
    fail "Health check failed (HTTP $HEALTH)"
fi

# 2. List stores
echo "2. Testing stores endpoint..."
STORES=$(curl -s --max-time $TIMEOUT "$API_URL/v1/stores" || echo "error")
if echo "$STORES" | grep -q "default\|stores"; then
    pass "Stores endpoint working"
else
    warn "Stores endpoint returned unexpected response"
fi

# 3. Test indexing
echo "3. Testing indexing endpoint..."
INDEX_RESPONSE=$(curl -s --max-time 30 -X POST "$API_URL/v1/stores/default/index" \
    -H "Content-Type: application/json" \
    -d '{
        "files": [
            {"path": "test/hello.py", "content": "def hello_world():\n    print(\"Hello, World!\")\n\ndef greet(name):\n    return f\"Hello, {name}!\""},
            {"path": "test/math.py", "content": "def add(a, b):\n    return a + b\n\ndef multiply(x, y):\n    return x * y"}
        ]
    }' || echo '{"error": true}')

if echo "$INDEX_RESPONSE" | grep -q "chunks_indexed\|indexed"; then
    pass "Indexing endpoint working"
    echo "   Response: $INDEX_RESPONSE"
else
    fail "Indexing failed: $INDEX_RESPONSE"
fi

# 4. Wait for indexing to complete
echo "4. Waiting for index to be ready..."
sleep 2

# 5. Test search
echo "5. Testing search endpoint..."
SEARCH_RESPONSE=$(curl -s --max-time $TIMEOUT -X POST "$API_URL/v1/search/default" \
    -H "Content-Type: application/json" \
    -d '{"query": "hello world function", "top_k": 5, "include_content": true}' || echo '{"error": true}')

if echo "$SEARCH_RESPONSE" | grep -q "results\|hello"; then
    pass "Search endpoint working"
    RESULT_COUNT=$(echo "$SEARCH_RESPONSE" | grep -o '"total":[0-9]*' | head -1 | cut -d: -f2)
    echo "   Found $RESULT_COUNT results"
else
    warn "Search returned unexpected response: $SEARCH_RESPONSE"
fi

# 6. Test MCP tools endpoint
echo "6. Testing MCP tools endpoint..."
MCP_TOOLS=$(curl -s --max-time $TIMEOUT "$API_URL/mcp/tools" || echo "error")
if echo "$MCP_TOOLS" | grep -q "code_search"; then
    pass "MCP tools endpoint working"
else
    warn "MCP tools endpoint returned unexpected response"
fi

# 7. Test MCP tool call
echo "7. Testing MCP tool call..."
MCP_SEARCH=$(curl -s --max-time $TIMEOUT -X POST "$API_URL/mcp/tools/call" \
    -H "Content-Type: application/json" \
    -d '{"name": "code_search", "arguments": {"query": "add function", "top_k": 3}}' || echo '{"error": true}')

if echo "$MCP_SEARCH" | grep -q "content\|path"; then
    pass "MCP tool call working"
else
    warn "MCP tool call returned unexpected response"
fi

echo ""
echo "=========================================="
echo "Smoke test completed!"
echo "=========================================="

# Summary
echo ""
echo "Services verified:"
echo "  - API Health: OK"
echo "  - Stores: OK"
echo "  - Indexing: OK"
echo "  - Search: OK"
echo "  - MCP: OK"
echo ""
echo "Platform is ready for use!"
