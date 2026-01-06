"""
Neural Reranker Service - unified-inference backend.

Uses unified-inference's rerank endpoint for dedicated reranker models,
or LLM-based reranking via chat as fallback.
"""
import logging
from typing import List, Dict, Any

from src.core.config import settings

logger = logging.getLogger(__name__)


async def rerank_results(query: str, documents: List[str]) -> List[float]:
    """
    Rerank documents using unified-inference (Async).

    Tries unified-inference rerank endpoint first, falls back to LLM-based.

    Args:
        query: The search query
        documents: List of document texts to rerank

    Returns:
        List of relevance scores (higher = more relevant)
    """
    if not documents:
        return []

    from src.services.inference import get_inference_client
    client = get_inference_client()

    mode = settings.RERANK_MODE.lower()
    logger.info(f"Reranking mode: {mode}")

    if mode == "tei" or mode == "rerank":
        # Use unified-inference's dedicated rerank endpoint
        try:
            results = await client.rerank(query, documents)
            scores = [r["score"] for r in results]
            logger.info(f"Unified-inference rerank returned {len(scores)} scores. Range: {min(scores):.3f} - {max(scores):.3f}")
            return scores
        except Exception as e:
            logger.warning(f"Unified-inference rerank failed, trying LLM: {e}")
    
    # LLM-based reranking via chat
    return await _rerank_with_llm(client, query, documents)


async def _rerank_with_llm(client, query: str, documents: List[str]) -> List[float]:
    """Rerank using LLM prompting (Async)."""
    try:
        docs_text = "\n".join([
            f"[{i+1}] {doc[:500]}"
            for i, doc in enumerate(documents)
        ])
        
        prompt = f"""Score each document's relevance to the query on a scale of 0-10.
Query: {query}

Documents:
{docs_text}

Return ONLY a comma-separated list of scores in order (e.g., "8,3,9,5").
Scores:"""

        response = await client.chat(
            messages=[{"role": "user", "content": prompt}],
            max_tokens=100,
            temperature=0.1
        )
        
        # Parse scores
        import re
        try:
            # Clean response: remove echoed prompt if present
            clean_response = response
            if "[/INST]" in response:
                clean_response = response.split("[/INST]")[-1]
            
            # 1. Try finding a explicit comma-separated list first (e.g. "8, 9, 7")
            # Look for at least 2 numbers separated by comma
            list_match = re.search(r"(\d+(?:\.\d+)?(?:\s*,\s*\d+(?:\.\d+)?)+)", clean_response)
            
            scores = []
            if list_match:
                # Parse the list found
                score_strs = list_match.group(1).split(",")
                scores = [float(s.strip()) for s in score_strs]
            else:
                # 2. Fallback: Find ALL numbers and try to interpret them
                # Use simple heuristic: filter out integers that look like indices (1, 2, 3 in sequence) if possible?
                # For now, just finding all numbers is risky but better than crashing.
                matches = re.findall(r"\b(\d+(?:\.\d+)?)\b", clean_response)
                scores = [float(m) for m in matches]

            # Normalize 0-10 -> 0-1
            # Heuristic: if any score > 1, assume 0-10 scale
            if any(s > 1.0 for s in scores):
                 scores = [s / 10.0 for s in scores]

            # Clamp and Truncate/Pad
            scores = [min(max(s, 0.0), 1.0) for s in scores]
            scores = scores[:len(documents)]
            
            while len(scores) < len(documents):
                scores.append(0.5)
            
            return scores

        except Exception as e:
            logger.warning(f"Could not parse LLM rerank scores: {e}. Response: {response[:100]}...")
            return [0.5] * len(documents)

            
    except Exception as e:
        logger.warning(f"LLM reranking unavailable, returning neutral scores: {e}")
        # Return neutral scores instead of failing - allows search without LLM
        return [0.5] * len(documents)


async def rerank_search_results(query: str, results: List[Dict[str, Any]], content_key: str = "text") -> List[Dict[str, Any]]:
    """Rerank search results and return sorted by relevance (Async)."""
    if not results:
        return results

    texts = [r.get(content_key, "") for r in results]
    logger.debug(f"Reranking {len(texts)} documents. First text sample: {texts[0][:100] if texts else 'N/A'}...")
    scores = await rerank_results(query, texts)
    logger.info(f"Raw rerank scores range: {min(scores):.3f} - {max(scores):.3f}")

    # BGE reranker returns raw confidence scores, typically in range [-1, 1]
    # but practically [0, 0.3] for most queries. We keep the raw scores as-is.
    # NO normalization - these are actual model confidence scores!

    for i, result in enumerate(results):
        result["rerank_score"] = scores[i]

    return sorted(results, key=lambda x: x.get("rerank_score", 0), reverse=True)
