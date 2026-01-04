"""
Result Fusion Algorithms.

Combines results from multiple retrievers (BM25, SPLADE, BM42) using:
- Reciprocal Rank Fusion (RRF)
- Weighted score fusion
"""

import logging
from typing import List, Dict, Any, Optional
from dataclasses import dataclass, field
from collections import defaultdict

logger = logging.getLogger(__name__)


@dataclass
class FusedResult:
    """Result after fusion from multiple retrievers."""
    chunk_id: str
    fused_score: float
    text: Optional[str] = None
    payload: Dict[str, Any] = field(default_factory=dict)
    sources: Dict[str, float] = field(default_factory=dict)  # retriever -> score


def rrf_fusion(
    result_sets: Dict[str, List[Dict]],
    limit: int = 10,
    k: int = 60
) -> List[FusedResult]:
    """
    Reciprocal Rank Fusion (RRF).
    
    Combines ranked lists by summing reciprocal ranks:
    RRF(d) = Σ 1/(k + rank(d))
    
    Args:
        result_sets: Dict mapping retriever name to list of results
                     Each result must have 'chunk_id' and 'score'
        limit: Maximum number of results to return
        k: RRF parameter (default 60)
        
    Returns:
        List of FusedResult objects, sorted by fused score
    """
    if not result_sets:
        return []
    
    # Aggregate RRF scores
    chunk_scores: Dict[str, float] = defaultdict(float)
    chunk_sources: Dict[str, Dict[str, float]] = defaultdict(dict)
    chunk_data: Dict[str, Dict] = {}
    
    for retriever_name, results in result_sets.items():
        for rank, result in enumerate(results):
            chunk_id = result.get("chunk_id") or result.get("id") or str(result.get("chunk_id", ""))
            if not chunk_id:
                continue
            
            # RRF score contribution
            rrf_score = 1.0 / (k + rank + 1)  # +1 because rank is 0-indexed
            chunk_scores[chunk_id] += rrf_score
            
            # Track source scores
            original_score = result.get("score", 0.0)
            chunk_sources[chunk_id][retriever_name] = original_score
            
            # Store payload (take first occurrence)
            if chunk_id not in chunk_data:
                chunk_data[chunk_id] = {
                    "text": result.get("text", ""),
                    "payload": {k: v for k, v in result.items() 
                               if k not in ("chunk_id", "id", "score", "text")}
                }
    
    # Sort by fused score and limit
    sorted_chunks = sorted(
        chunk_scores.items(),
        key=lambda x: x[1],
        reverse=True
    )[:limit]
    
    # Build results
    results = []
    for chunk_id, fused_score in sorted_chunks:
        data = chunk_data.get(chunk_id, {})
        results.append(FusedResult(
            chunk_id=chunk_id,
            fused_score=fused_score,
            text=data.get("text"),
            payload=data.get("payload", {}),
            sources=chunk_sources[chunk_id]
        ))
    
    return results


def weighted_fusion(
    result_sets: Dict[str, List[Dict]],
    weights: Dict[str, float] = None,
    limit: int = 10
) -> List[FusedResult]:
    """
    Weighted score fusion.
    
    Combines results by weighted sum of normalized scores.
    
    Args:
        result_sets: Dict mapping retriever name to list of results
        weights: Dict mapping retriever name to weight (default: equal weights)
        limit: Maximum number of results to return
        
    Returns:
        List of FusedResult objects, sorted by fused score
    """
    if not result_sets:
        return []
    
    # Default equal weights
    if weights is None:
        weights = {name: 1.0 for name in result_sets.keys()}
    
    # Normalize weights
    total_weight = sum(weights.values())
    weights = {k: v / total_weight for k, v in weights.items()}
    
    # Normalize scores within each result set
    normalized_sets = {}
    for retriever_name, results in result_sets.items():
        if not results:
            continue
            
        scores = [r.get("score", 0.0) for r in results]
        max_score = max(scores) if scores else 1.0
        min_score = min(scores) if scores else 0.0
        score_range = max_score - min_score if max_score != min_score else 1.0
        
        normalized = []
        for r in results:
            norm_score = (r.get("score", 0.0) - min_score) / score_range
            normalized.append({**r, "norm_score": norm_score})
        
        normalized_sets[retriever_name] = normalized
    
    # Aggregate weighted scores
    chunk_scores: Dict[str, float] = defaultdict(float)
    chunk_sources: Dict[str, Dict[str, float]] = defaultdict(dict)
    chunk_data: Dict[str, Dict] = {}
    
    for retriever_name, results in normalized_sets.items():
        weight = weights.get(retriever_name, 0.0)
        
        for result in results:
            chunk_id = result.get("chunk_id") or result.get("id", "")
            if not chunk_id:
                continue
            
            # Weighted normalized score
            weighted_score = result.get("norm_score", 0.0) * weight
            chunk_scores[chunk_id] += weighted_score
            
            # Track source scores
            chunk_sources[chunk_id][retriever_name] = result.get("score", 0.0)
            
            # Store payload
            if chunk_id not in chunk_data:
                chunk_data[chunk_id] = {
                    "text": result.get("text", ""),
                    "payload": {k: v for k, v in result.items() 
                               if k not in ("chunk_id", "id", "score", "norm_score", "text")}
                }
    
    # Sort and limit
    sorted_chunks = sorted(
        chunk_scores.items(),
        key=lambda x: x[1],
        reverse=True
    )[:limit]
    
    # Build results
    results = []
    for chunk_id, fused_score in sorted_chunks:
        data = chunk_data.get(chunk_id, {})
        results.append(FusedResult(
            chunk_id=chunk_id,
            fused_score=fused_score,
            text=data.get("text"),
            payload=data.get("payload", {}),
            sources=chunk_sources[chunk_id]
        ))
    
    return results


def deduplicate_results(results: List[Dict], key: str = "chunk_id") -> List[Dict]:
    """Remove duplicate results by key."""
    seen = set()
    unique = []
    for r in results:
        k = r.get(key)
        if k and k not in seen:
            seen.add(k)
            unique.append(r)
    return unique
