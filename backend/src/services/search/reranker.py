"""
Neural Reranker Service - Xinference backend.

Uses Xinference's rerank endpoint for dedicated reranker models,
or LLM-based reranking via chat as fallback.
"""
import logging
from typing import List, Dict, Any

from src.core.config import settings

logger = logging.getLogger(__name__)


def rerank_results(query: str, documents: List[str]) -> List[float]:
    """
    Rerank documents using Xinference.
    
    Tries Xinference rerank endpoint first, falls back to LLM-based.
    
    Args:
        query: The search query
        documents: List of document texts to rerank
        
    Returns:
        List of relevance scores (higher = more relevant)
    """
    if not documents:
        return []
    
    from src.services.inference import get_xinference_client
    client = get_xinference_client()
    
    mode = settings.RERANK_MODE.lower()
    
    if mode == "tei" or mode == "rerank":
        # Use Xinference's dedicated rerank endpoint
        try:
            return client.rerank(query, documents)
        except Exception as e:
            logger.warning(f"Xinference rerank failed, trying LLM: {e}")
    
    # LLM-based reranking via chat
    return _rerank_with_llm(client, query, documents)


def _rerank_with_llm(client, query: str, documents: List[str]) -> List[float]:
    """Rerank using LLM prompting."""
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

        response = client.chat(
            messages=[{"role": "user", "content": prompt}],
            max_tokens=100,
            temperature=0.1
        )
        
        # Parse scores
        try:
            score_strs = response.strip().split(",")
            scores = [float(s.strip()) / 10.0 for s in score_strs[:len(documents)]]
            while len(scores) < len(documents):
                scores.append(0.5)
            return scores
        except ValueError:
            logger.warning(f"Could not parse LLM rerank scores: {response}")
            return [0.5] * len(documents)
            
    except Exception as e:
        logger.warning(f"LLM reranking unavailable, returning neutral scores: {e}")
        # Return neutral scores instead of failing - allows search without LLM
        return [0.5] * len(documents)


def rerank_search_results(query: str, results: List[Dict[str, Any]], content_key: str = "content") -> List[Dict[str, Any]]:
    """Rerank search results and return sorted by relevance."""
    if not results:
        return results
    
    texts = [r.get(content_key, "") for r in results]
    scores = rerank_results(query, texts)
    
    for i, result in enumerate(results):
        result["rerank_score"] = scores[i]
    
    return sorted(results, key=lambda x: x.get("rerank_score", 0), reverse=True)
