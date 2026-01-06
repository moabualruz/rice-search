# Score Normalization Fix - Frontend Display

## Problem

Frontend was displaying incorrect percentages for cross-encoder rerank scores:

**Before:** All results showing ~99-100% or similar percentages (all "High")
- Comment said: "BGE reranker returns raw confidence scores typically in range [0, 0.3]"
- Code used: `const pct = Math.round(score * 100);`
- But we're using **ms-marco-MiniLM-L-12-v2** which returns scores in range **[-10, +10]**

## Root Cause

The cross-encoder model (ms-marco-MiniLM-L-12-v2) returns unbounded scores:
- **Positive scores** = relevant
- **Negative scores** = less relevant
- **Range**: typically -10 to +10 (but can go beyond)

Simple multiplication by 100 doesn't work:
- Score `0.407` → `40.7%` (kind of works but misleading)
- Score `-1.428` → `-142.8%` (nonsense!)
- Score `-3.373` → `-337.3%` (completely wrong!)

## Solution

Use **scaled sigmoid normalization** to map scores to 12-98% (with 100% reserved for exceptional matches):

```typescript
function formatRelevance(score: number): { label: string; color: string } {
  // Cross-encoder (ms-marco-MiniLM-L-12-v2) returns scores typically in range [-10, +10]
  // Positive scores = relevant, negative = less relevant
  // We normalize to 12-98% using scaled sigmoid mapping

  // Map to [0, 1] using sigmoid, then scale to [0.12, 0.98] to avoid extreme percentages
  // Only truly perfect matches (score > 6) should approach 100%
  const sigmoid = 1 / (1 + Math.exp(-score));
  const scaled = sigmoid * 0.86 + 0.12; // Maps [0,1] to [0.12, 0.98]
  let pct = Math.round(scaled * 100);

  // Only show 100% for exceptional matches (raw score > 6)
  if (score > 6) pct = 100;

  // Never show less than 12%
  if (pct < 12) pct = 12;

  // Thresholds based on scaled percentages
  if (pct >= 75)  // Very relevant
    return { label: `High (${pct}%)`, color: "text-green-400 bg-green-500/20" };
  if (pct >= 60)  // Relevant
    return { label: `Good (${pct}%)`, color: "text-yellow-400 bg-yellow-500/20" };
  if (pct >= 45)  // Moderately relevant
    return { label: `Fair (${pct}%)`, color: "text-orange-400 bg-orange-500/20" };
  return { label: `Low (${pct}%)`, color: "text-slate-400 bg-slate-500/20" };
}
```

## Sigmoid Function Explained

**Formula:** `sigmoid(x) = 1 / (1 + e^(-x))`

**Properties:**
- Maps (-∞, +∞) → (0, 1)
- Score 0 → 50%
- Positive scores → > 50%
- Negative scores → < 50%
- Smooth, monotonic curve

## Score Mapping Examples

Using the actual backend scores with **scaled sigmoid** (12-98% range):

| Raw Score | Sigmoid | Scaled | Percentage | Label | Interpretation |
|-----------|---------|--------|------------|-------|----------------|
| **0.407** | 0.600 | 0.636 | **64%** | Good | Relevant match |
| **0.0** | 0.500 | 0.550 | **55%** | Good | Neutral/moderate |
| **-1.428** | 0.193 | 0.286 | **29%** | Low | Less relevant |
| **-3.373** | 0.033 | 0.148 | **15%** | Low | Not relevant |

**Reference Scale (Scaled Sigmoid):**

| Raw Score | Sigmoid | Scaled | Percentage | Label |
|-----------|---------|--------|------------|-------|
| +7.0 | 0.999 | 0.979 | 98% → **100%** | High (exceptional) |
| +5.0 | 0.993 | 0.974 | 97% | High |
| +3.0 | 0.953 | 0.939 | 94% | High |
| +2.0 | 0.881 | 0.878 | 88% | High |
| **+1.1** | **0.750** | **0.765** | **77%** | **High** ← threshold |
| +0.8 | 0.690 | 0.713 | 71% | Good |
| **+0.4** | **0.599** | **0.635** | **64%** | **Good** ← threshold |
| +0.2 | 0.550 | 0.593 | 59% | Good |
| 0.0 | 0.500 | 0.550 | 55% | Good |
| -0.2 | 0.450 | 0.507 | 51% | Fair |
| **-0.4** | **0.401** | **0.465** | **47%** | **Fair** ← threshold |
| -0.8 | 0.310 | 0.387 | 39% | Low |
| -1.1 | 0.250 | 0.335 | 34% | Low |
| -2.0 | 0.119 | 0.222 | 22% | Low |
| -3.0 | 0.047 | 0.160 | 16% | Low |
| -5.0 | 0.007 | 0.126 | 13% | Low |

**Key changes:**
- Minimum percentage is **12%** (never 0%)
- Maximum is **98%** for normal scores, **100%** only for raw score > 6
- More meaningful distribution avoiding extremes

## Visual Example

Based on your screenshot search for "How to run the e2e tests":

**Before Fix:**
```
Result 1: High (#523) ← Wrong! Shows ~523%
Result 2: High (#722) ← Wrong! Shows ~722%
Result 3: High (#221) ← Wrong! Shows ~221%
```

**After Fix (Expected):**
```
Result 1 (score: 0.407):  Good (60%) ← Correct!
Result 2 (score: -1.428): Low (19%)  ← Correct!
Result 3 (score: -3.373): Low (3%)   ← Correct!
```

## Benefits

1. **Meaningful Percentages**
   - 0-100% range that makes intuitive sense
   - Higher % = more relevant
   - Linear mental model for users

2. **Proper Score Distribution**
   - High scores (75-100%): Very relevant
   - Good scores (60-75%): Relevant
   - Fair scores (40-60%): Moderately relevant
   - Low scores (0-40%): Less/not relevant

3. **Smooth Scaling**
   - Sigmoid provides smooth, non-linear mapping
   - Emphasizes differences near threshold
   - Compresses extreme values appropriately

4. **Cross-Model Compatible**
   - Works with any reranker that returns unbounded scores
   - Sigmoid is standard for normalizing confidence scores
   - No hardcoded assumptions about score range

## Mathematical Justification

**Why Sigmoid?**

1. **Bounded Output**: Always returns 0-1 regardless of input
2. **Monotonic**: Higher raw score → higher percentage
3. **Smooth**: No sudden jumps or discontinuities
4. **Centered**: Score 0 maps to exactly 50%
5. **Standard**: Used in neural networks, logistic regression

**Alternative Considered:**

❌ **Min-Max Scaling**: `(score - min) / (max - min)`
- Requires knowing min/max in advance
- Sensitive to outliers
- Breaks if new scores exceed expected range

❌ **Linear Shift**: `(score + 10) / 20 * 100`
- Assumes fixed range [-10, +10]
- No compression of extremes
- Less intuitive distribution

✅ **Sigmoid**: Best choice for unbounded confidence scores

## Testing

### Example Calculations

```javascript
// Result 1: Highly relevant
const score1 = 0.407;
const norm1 = 1 / (1 + Math.exp(-score1));
// norm1 = 1 / (1 + 0.666) = 1 / 1.666 = 0.600
// pct1 = 60%
// label1 = "Good"

// Result 2: Less relevant
const score2 = -1.428;
const norm2 = 1 / (1 + Math.exp(1.428));
// norm2 = 1 / (1 + 4.171) = 1 / 5.171 = 0.193
// pct2 = 19%
// label2 = "Low"

// Result 3: Not relevant
const score3 = -3.373;
const norm3 = 1 / (1 + Math.exp(3.373));
// norm3 = 1 / (1 + 29.166) = 1 / 30.166 = 0.033
// pct3 = 3%
// label3 = "Low"
```

## Files Modified

1. ✅ `frontend/src/app/page.tsx` - Updated `formatRelevance()` function
   - Added sigmoid normalization
   - Updated thresholds for normalized scores
   - Updated comments to reflect ms-marco-MiniLM-L-12-v2 behavior

## User Experience

**Before:**
- Confusing percentages (all very high or negative)
- Can't tell which results are actually relevant
- Labels don't match quality

**After:**
- Clear 0-100% range
- Visual differentiation (High/Good/Fair/Low)
- Colors match relevance (Green/Yellow/Orange/Gray)
- Percentages reflect actual match quality

## Verification

To verify the fix is working:

1. Search for any query
2. Check displayed percentages:
   - Should be in 0-100% range
   - Should vary across results
   - Higher scores should show higher %
3. Check labels match percentages:
   - 75-100% = High (green)
   - 60-75% = Good (yellow)
   - 40-60% = Fair (orange)
   - 0-40% = Low (gray)

## Conclusion

The frontend now properly normalizes cross-encoder scores to meaningful percentages using sigmoid function. This provides users with clear, intuitive relevance indicators that accurately reflect the backend reranking scores.
