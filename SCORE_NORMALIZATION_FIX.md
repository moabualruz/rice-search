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

Use **sigmoid normalization** to map scores to 0-100%:

```typescript
function formatRelevance(score: number): { label: string; color: string } {
  // Cross-encoder (ms-marco-MiniLM-L-12-v2) returns scores typically in range [-10, +10]
  // Positive scores = relevant, negative = less relevant
  // We normalize to 0-100% using sigmoid-like mapping

  // Map [-10, +10] to [0, 1] using sigmoid function
  // This gives smooth 0-100% range with 50% at score=0
  const normalized = 1 / (1 + Math.exp(-score));
  const pct = Math.round(normalized * 100);

  // Thresholds based on normalized scores
  if (normalized >= 0.75)  // score > ~1.1
    return { label: `High (${pct}%)`, color: "text-green-400 bg-green-500/20" };
  if (normalized >= 0.60)  // score > ~0.4
    return { label: `Good (${pct}%)`, color: "text-yellow-400 bg-yellow-500/20" };
  if (normalized >= 0.40)  // score > -0.4
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

Using the actual backend scores from your search results:

| Raw Score | Sigmoid | Percentage | Label | Interpretation |
|-----------|---------|------------|-------|----------------|
| **0.407** | 0.600 | **60%** | Good | Relevant match |
| **0.0** | 0.500 | **50%** | Fair | Neutral |
| **-1.428** | 0.193 | **19%** | Low | Less relevant |
| **-3.373** | 0.033 | **3%** | Low | Not relevant |

**Reference Scale:**

| Raw Score | Sigmoid | Percentage | Label |
|-----------|---------|------------|-------|
| +5.0 | 0.993 | 99% | High |
| +3.0 | 0.953 | 95% | High |
| +2.0 | 0.881 | 88% | High |
| **+1.1** | **0.750** | **75%** | **High** ← threshold |
| +0.8 | 0.690 | 69% | Good |
| **+0.4** | **0.599** | **60%** | **Good** ← threshold |
| +0.2 | 0.550 | 55% | Good |
| 0.0 | 0.500 | 50% | Fair |
| -0.2 | 0.450 | 45% | Fair |
| **-0.4** | **0.401** | **40%** | **Fair** ← threshold |
| -0.8 | 0.310 | 31% | Low |
| -1.1 | 0.250 | 25% | Low |
| -2.0 | 0.119 | 12% | Low |
| -3.0 | 0.047 | 5% | Low |
| -5.0 | 0.007 | 1% | Low |

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
