# File Path Indexing Enhancement

## Overview

Enhanced the indexing pipeline to make **file paths and file names searchable**, not just metadata. This allows users to search for files by name (e.g., "config.yaml", "README.md", "test_api_files.py") and get relevant results.

## Changes Made

### 1. Enhanced Content for Embedding (`backend/src/services/ingestion/indexer.py`)

**Before:** Only chunk content was embedded
```python
contents = [c["content"] for c in chunks]  # Raw content only
dense_embeddings = embed_texts(contents)
```

**After:** File path/name prepended to content before embedding
```python
# Extract file name
file_name = os.path.basename(display_path)

# Enhance content with file metadata
enhanced_contents = []
for c in chunks:
    enhanced = f"File: {file_name}\nPath: {display_path}\n\n{c['content']}"
    enhanced_contents.append(enhanced)

# Embed enhanced content
dense_embeddings = embed_texts(enhanced_contents)
```

**What gets embedded:**
```
File: config.yaml
Path: /backend/src/core/config.py

[original chunk content here]
```

### 2. Added Metadata Fields

**Before:** Only `file_path` and `client_system_path` in metadata
```python
payload={
    "text": chunk["content"],
    "file_path": display_path,  # Metadata only
    ...
}
```

**After:** Added `full_path` and `filename` as dedicated fields
```python
payload={
    "text": chunk["content"],  # Original content (not enhanced)
    **chunk["metadata"],
    "full_path": display_path,   # Full path for filtering
    "filename": file_name,        # Just filename for quick access
    ...
}
```

**Metadata structure:**
```json
{
  "text": "actual chunk content",
  "file_path": "/backend/src/core/config.py",
  "client_system_path": "/backend/src/core/config.py",
  "full_path": "/backend/src/core/config.py",
  "filename": "config.py",
  "org_id": "public",
  "doc_id": "...",
  "chunk_id": "...",
  ...
}
```

### 3. Updated CLAUDE.md

Added clear guidelines:
- **ALWAYS use `ricesearch.exe` CLI for indexing**
- Never use direct API calls for indexing
- Reindex after schema changes with `ricesearch watch`

## Benefits

### 1. **File Name Searches Work**

**Now these searches work:**
```bash
# Search for config files
ricesearch search "config.yaml"

# Search for README
ricesearch search "README"

# Search for test files
ricesearch search "test_api_files.py"

# Search for specific file types
ricesearch search "dockerfile"
ricesearch search "package.json"
```

**Results will:**
- Rank files with matching names higher
- Show relevant chunks from those files
- Include file path in results for easy identification

### 2. **Better Semantic Understanding**

The embedding models now understand file context:
- "authentication.py" in content → model knows it's about auth code
- "test_" prefix → model understands it's test code
- File extensions → model can infer language/type

### 3. **Improved Filtering**

With dedicated `full_path` and `filename` fields:
```python
# Filter by filename
results = qdrant.scroll(
    collection_name="...",
    scroll_filter=Filter(
        must=[FieldCondition(key="filename", match=MatchValue(value="config.py"))]
    )
)

# Filter by path prefix
results = qdrant.scroll(
    collection_name="...",
    scroll_filter=Filter(
        must=[FieldCondition(key="full_path", match=MatchText(text="/backend/src/"))]
    )
)
```

### 4. **Consistent Path Handling**

- `full_path`: Always contains complete path for UI display
- `filename`: Just the filename for quick access
- `file_path` and `client_system_path`: Legacy compatibility

## Impact on Storage

**Storage increase:**
- Enhanced content adds ~20-100 bytes per chunk (file path length)
- For a typical file with 10 chunks: +200-1000 bytes
- For 10,000 files: ~2-10 MB additional storage
- **Negligible** compared to embedding vectors (768-1024 floats × 10,000 chunks = ~30-40 MB)

**Embedding cost:**
- Each chunk embedding now includes file path (~10-30 tokens)
- Minimal impact on embedding time
- No impact on search speed (vectors are same size)

## Indexing Pipeline Flow

```
File: /backend/src/core/config.py
│
├─ Parse file → Extract content
│
├─ Chunk content → 5 chunks
│
├─ Enhance each chunk:
│   Chunk 1: "File: config.py\nPath: /backend/src/core/config.py\n\n[content1]"
│   Chunk 2: "File: config.py\nPath: /backend/src/core/config.py\n\n[content2]"
│   ...
│
├─ Generate embeddings for enhanced content
│
├─ Store in Qdrant:
│   {
│     "vectors": {"dense": [...], "splade": [...], "bm42": [...]},
│     "payload": {
│       "text": "[original content]",  ← Display to user
│       "full_path": "/backend/src/core/config.py",
│       "filename": "config.py",
│       ...
│     }
│   }
│
└─ Index in Tantivy (BM25):
    Enhanced content → BM25 index
```

## Reindexing Instructions

After deploying this enhancement, **reindex all files** to enable file name searching:

### Using ricesearch CLI (Recommended)

```bash
# Navigate to project root
cd F:/work/rice-search

# Reindex backend code
ricesearch watch ./backend --org-id public

# Let it complete initial scan, then Ctrl+C to stop
# OR keep running for continuous monitoring
```

### What Happens During Reindex

1. **Scans all files** in the directory
2. **Computes hash** for each file
3. **Indexes new/changed files** via API
4. **Skips unchanged files** (based on hash)
5. **Updates embeddings** with file path metadata

**Expected output:**
```
Watching: F:\work\rice-search\backend
Initial scan...
Indexing: backend\src\core\config.py
✓ Indexed 12 chunks
Indexing: backend\src\services\ingestion\indexer.py
✓ Indexed 25 chunks
...
Scanned 247 files
Watching for changes... (Ctrl+C to stop)
```

## Testing

### Verify File Name Search Works

```bash
# Search for a specific file
ricesearch search "config.py" --limit 5

# Expected: Results from config.py files ranked highly
```

### Verify Metadata Fields

```bash
curl -X POST http://localhost:8000/api/v1/search/query \
  -H "Content-Type: application/json" \
  -d '{"query": "config", "limit": 1, "mode": "search"}'
```

**Expected response includes:**
```json
{
  "results": [
    {
      "text": "actual content...",
      "full_path": "/backend/src/core/config.py",
      "filename": "config.py",
      "file_path": "/backend/src/core/config.py",
      "client_system_path": "/backend/src/core/config.py",
      ...
    }
  ]
}
```

## Backward Compatibility

✅ **Fully backward compatible**
- Existing metadata fields retained (`file_path`, `client_system_path`)
- Old documents still searchable
- Frontend continues to work with existing results
- New fields optional for old documents

## Future Enhancements

1. **Path-based filtering in UI**
   - Filter results by directory
   - Filter by filename pattern
   - Use `full_path` and `filename` fields

2. **Smart file ranking**
   - Boost results from files matching query exactly
   - Rank by path similarity
   - Prefer files with matching extensions

3. **File type indicators**
   - Extract extension from `filename`
   - Show file type icons in UI
   - Filter by file type

4. **Path autocomplete**
   - Use `full_path` for autocomplete
   - Suggest files as user types
   - Navigate directory structure

## Files Modified

1. ✅ `backend/src/services/ingestion/indexer.py`
   - Lines 217-230: Enhanced content with file path/name
   - Lines 300-301: Added `full_path` and `filename` to payload

2. ✅ `CLAUDE.md`
   - Updated CLI guidelines
   - Added indexing best practices
   - Emphasized using `ricesearch.exe` for all indexing

## Summary

The indexing pipeline now:
1. **Includes file paths in embeddings** → File names are searchable
2. **Stores full_path and filename fields** → Easy filtering and display
3. **Uses CLI for all indexing** → Consistent path handling
4. **Maintains backward compatibility** → No breaking changes

**Action Required:** Reindex your codebase with `ricesearch watch` to enable file name searching!
