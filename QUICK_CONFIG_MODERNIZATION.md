# Quick Config Modal Modernization

## Overview

Modernized the Quick Config modal in the admin dashboard to use the centralized settings API and include only important, actively connected settings. The modal now provides instant access to the most critical system configuration options with automatic persistence.

## Implementation Date

2026-01-06

## Changes Made

### File Modified

**`frontend/src/app/admin/page.tsx`** - Complete rewrite of `SettingsModal` component

### Key Improvements

1. **Centralized Settings Integration**
   - Now uses `/api/v1/settings/` REST API
   - All changes automatically persist to both Redis and `settings.yaml`
   - Real-time updates with individual setting saves
   - No manual save required - instant persistence

2. **Removed Unused Settings**
   - ❌ Removed MCP Protocol settings (not actively used)
   - ❌ Removed worker pool/concurrency settings (system-level config)
   - ✅ Focused only on user-facing, important settings

3. **Added Important Settings**
   - **Triple Retrieval System**
     - BM25 Lexical Search toggle (`search.hybrid.use_bm25`)
     - SPLADE Neural Sparse toggle (`search.hybrid.use_splade`)
     - BM42 Hybrid Search toggle (`search.hybrid.use_bm42`)

   - **Search Enhancements**
     - Neural Reranking toggle (`models.reranker.enabled`)
     - AST-Aware Parsing toggle (`ast.enabled`)

   - **Performance Tuning**
     - RRF Constant (k) - Rank fusion balance (`search.hybrid.rrf_k`)
     - Default Result Limit (`search.default_limit`)
     - Chunk Size in characters (`indexing.chunk_size`)

   - **Memory Optimization**
     - Auto-Unload Models toggle (`model_management.auto_unload`)
     - Idle Timeout in seconds (`model_management.ttl_seconds`)

### UI Features

1. **Organized Sections**
   - 🔍 Triple Retrieval System - Control all three retrieval methods
   - ⚡ Search Enhancements - Toggle advanced features
   - 🎛️ Performance Tuning - Fine-tune search parameters
   - Memory Optimization subsection within tuning

2. **Type-Aware Controls**
   - Toggle switches for boolean settings (BM25, SPLADE, reranker, etc.)
   - Number inputs for numeric settings (RRF k, limits, chunk size, TTL)
   - Tooltips with explanations (e.g., RRF constant tooltip)
   - Conditional rendering (TTL input only shows when auto-unload is enabled)

3. **User Experience**
   - Loading state while fetching settings
   - Auto-save on every change (no save button needed)
   - Message: "💡 Changes are automatically saved to settings.yaml"
   - Clean, modern dark theme UI with Tailwind CSS
   - Hover effects and smooth transitions

## Technical Implementation

### Settings Fetch

```typescript
const keys = [
  'search.hybrid.use_bm25',
  'search.hybrid.use_splade',
  'search.hybrid.use_bm42',
  'models.reranker.enabled',
  'ast.enabled',
  'search.hybrid.rrf_k',
  'search.default_limit',
  'indexing.chunk_size',
  'model_management.auto_unload',
  'model_management.ttl_seconds',
];

await Promise.all(
  keys.map(async (key) => {
    const res = await fetch(`${SETTINGS_API}/${key}`);
    if (res.ok) {
      const data = await res.json();
      results[key] = data.value;
    }
  })
);
```

### Settings Update

```typescript
const updateSetting = async (key: string, value: any) => {
  const res = await fetch(`${SETTINGS_API}/${key}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ value })  // No persist parameter - auto persists
  });

  if (res.ok) {
    setSettings(prev => ({ ...prev, [key]: value }));
  }
};
```

## Benefits

1. **Simplified Configuration**
   - Most important settings in one modal
   - No need to navigate to full settings page for quick changes
   - Instant feedback and persistence

2. **Better UX**
   - No manual save required
   - Clear organization by category
   - Descriptive labels and tooltips
   - Visual indicators (toggle switches, number badges)

3. **Safer Defaults**
   - All changes persist automatically
   - No risk of losing configuration
   - Settings survive container restarts

4. **Performance**
   - Individual setting updates (no bulk payload)
   - Parallel fetching on load
   - Minimal API calls

## Testing Verification

### Tested Scenarios

1. **Settings Load** ✓
   ```bash
   curl http://localhost:8000/api/v1/settings/search.hybrid.use_bm25
   # {"key":"search.hybrid.use_bm25","value":true}
   ```

2. **Settings Update** ✓
   ```bash
   curl -X PUT http://localhost:8000/api/v1/settings/search.default_limit \
     -H "Content-Type: application/json" -d '{"value": 25}'
   # {"message":"Setting updated and persisted to file","persisted":true,"version":10}
   ```

3. **Auto-Persistence** ✓
   ```bash
   grep "default_limit" backend/settings.yaml
   # default_limit: 25  ← Updated immediately
   ```

4. **Frontend Compilation** ✓
   ```
   ✓ Ready in 2.4s
   ✓ Compiled / in 3.5s (2140 modules)
   ```

5. **Backend Health** ✓
   ```bash
   curl http://localhost:8000/health
   # {"status":"ok"}
   ```

## Access

1. Navigate to admin dashboard: `http://localhost:3000/admin`
2. Click "🎛️ Quick Config" button in the header
3. Modal opens with all important settings
4. Toggle switches or adjust numbers
5. Changes save automatically
6. Close modal when done

## Integration with Full Settings Editor

The Quick Config modal complements the full settings editor at `/admin/settings`:

- **Quick Config** - Fast access to 10 most important settings
- **Full Editor** - Complete settings browser with 100+ settings

Both use the same centralized settings API and auto-persist to `settings.yaml`.

## Future Enhancements

- [ ] Add preset profiles (e.g., "Fast Search", "Balanced", "High Quality")
- [ ] Show recommended values based on system resources
- [ ] Display current resource usage (GPU memory, RAM)
- [ ] Add "Reset to Defaults" button for individual settings
- [ ] Live preview of setting impact (e.g., show RRF k effect on results)

## Related Documentation

- `SETTINGS_AUTO_PERSIST.md` - Auto-persistence implementation
- `SETTINGS_SYSTEM.md` - Complete settings system documentation
- `frontend/src/app/admin/settings/page.tsx` - Full settings editor

## Conclusion

The Quick Config modal has been successfully modernized to use the centralized settings API and includes only the most important, actively connected settings. All changes persist automatically to the YAML file, providing a seamless configuration experience for users.
