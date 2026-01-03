"""
Query Analyzer Service - vLLM-based.

ALL query classification is delegated to vLLM (CodeLlama).
Uses pattern matching for fast analysis, with vLLM for complex queries.
No in-process model loading - backend only orchestrates service calls.
"""

import logging
import re
from typing import Dict, Any, Optional, List
from dataclasses import dataclass
from enum import Enum

logger = logging.getLogger(__name__)


class QueryIntent(str, Enum):
    """Query intent types."""
    LOOKUP = "lookup"           # Find specific code/function
    EXPLANATION = "explanation"  # Understand how something works
    NAVIGATION = "navigation"    # Find where something is defined
    EXAMPLE = "example"          # Find usage examples
    DEBUG = "debug"              # Find error-related code


@dataclass
class QueryAnalysis:
    """Result of query analysis."""
    original_query: str
    processed_query: str
    intent: QueryIntent
    confidence: float
    language_hints: List[str]
    path_hints: List[str]
    symbol_hints: List[str]
    filters: Dict[str, Any]


# Pattern-based hints (fast path)
INTENT_PATTERNS = {
    QueryIntent.LOOKUP: [
        r"\bwhere\s+is\b", r"\bfind\b", r"\bget\b", r"\bwhat\s+is\b",
        r"\bdefinition\s+of\b", r"\bfunction\b.*\bnamed\b"
    ],
    QueryIntent.EXPLANATION: [
        r"\bhow\s+does\b", r"\bwhy\b", r"\bexplain\b", r"\bunderstand\b",
        r"\bwhat\s+does\b.*\bdo\b"
    ],
    QueryIntent.NAVIGATION: [
        r"\bwhere\b.*\bdefined\b", r"\bimport\b", r"\bfile\b.*\bcontains\b",
        r"\bin\s+which\b"
    ],
    QueryIntent.EXAMPLE: [
        r"\bexample\b", r"\bhow\s+to\b", r"\busage\b", r"\buse\b.*\bto\b"
    ],
    QueryIntent.DEBUG: [
        r"\berror\b", r"\bbug\b", r"\bfail\b", r"\bcrash\b", r"\bexception\b",
        r"\bfix\b", r"\bdebug\b"
    ]
}

LANGUAGE_KEYWORDS = {
    "py": ["python", "py", ".py", "def ", "import ", "class "],
    "js": ["javascript", "js", ".js", "function ", "const ", "let ", "=>"],
    "ts": ["typescript", "ts", ".ts", "interface ", "type "],
    "go": ["golang", "go", ".go", "func ", "package "],
    "rs": ["rust", "rs", ".rs", "fn ", "impl ", "mod "],
    "java": ["java", ".java", "class ", "public ", "private "],
}


def analyze_query(query: str, use_llm: bool = False) -> QueryAnalysis:
    """
    Analyze a search query to extract intent and hints.
    
    Args:
        query: The search query
        use_llm: If True, use vLLM for complex classification
        
    Returns:
        QueryAnalysis with extracted information
    """
    query_lower = query.lower()
    
    # 1. Pattern-based intent detection (fast path)
    intent = QueryIntent.LOOKUP  # Default
    confidence = 0.5
    
    for intent_type, patterns in INTENT_PATTERNS.items():
        for pattern in patterns:
            if re.search(pattern, query_lower):
                intent = intent_type
                confidence = 0.8
                break
        if confidence > 0.5:
            break
    
    # 2. Language hints
    language_hints = []
    for lang, keywords in LANGUAGE_KEYWORDS.items():
        for kw in keywords:
            if kw in query_lower:
                language_hints.append(lang)
                break
    
    # 3. Path hints (quoted paths or file extensions)
    path_hints = re.findall(r'["\']([^"\']+\.[a-z]+)["\']', query)
    path_hints.extend(re.findall(r'\b(\w+\.[a-z]{2,4})\b', query))
    
    # 4. Symbol hints (camelCase, snake_case, PascalCase)
    symbol_hints = re.findall(r'\b([a-z]+[A-Z][a-zA-Z]*|[a-z]+_[a-z_]+)\b', query)
    symbol_hints.extend(re.findall(r'\b([A-Z][a-zA-Z]+)\b', query))
    
    # 5. Optional: Use vLLM for complex classification
    if use_llm and confidence < 0.7:
        try:
            llm_intent = classify_with_llm(query)
            if llm_intent:
                intent = llm_intent
                confidence = 0.9
        except Exception as e:
            logger.warning(f"vLLM classification failed: {e}")
    
    # Build filters from hints
    filters = {}
    if language_hints:
        filters["language"] = language_hints
    if path_hints:
        filters["path_pattern"] = path_hints[0] if len(path_hints) == 1 else path_hints
    
    return QueryAnalysis(
        original_query=query,
        processed_query=query,  # Could be cleaned/expanded
        intent=intent,
        confidence=confidence,
        language_hints=language_hints,
        path_hints=path_hints,
        symbol_hints=symbol_hints,
        filters=filters
    )


def classify_with_llm(query: str) -> Optional[QueryIntent]:
    """
    Classify query intent using vLLM (CodeLlama).
    
    Uses vLLM's chat API with a classification prompt.
    
    Raises:
        RuntimeError: If vLLM service is unavailable
    """
    from src.services.inference import get_vllm_client
    
    categories = [intent.value for intent in QueryIntent]
    
    try:
        client = get_vllm_client()
        result = client.classify_query(query, categories)
        
        # Parse result
        result_lower = result.lower().strip()
        for intent in QueryIntent:
            if intent.value in result_lower:
                return intent
        
        return None
    except Exception as e:
        logger.error(f"vLLM classification failed: {e}")
        raise RuntimeError(f"Query classification service unavailable: {e}")


# For backwards compatibility
def get_query_analyzer():
    """Get query analyzer (pattern-based, no model loading)."""
    return analyze_query


# Removed: QueryAnalyzer class with in-process model loading
# Removed: _get_model() - all LLM inference via vLLM
