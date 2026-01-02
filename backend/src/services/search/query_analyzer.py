"""
Query Analyzer Service (REQ-SRCH-01).

Analyzes search queries to determine intent and extract scope filters.
Uses CodeBERT for code-aware query understanding.
"""

import logging
import re
from typing import Dict, Any, Optional, List, Tuple
from dataclasses import dataclass
from enum import Enum

from src.core.config import settings

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


class QueryAnalyzer:
    """
    Analyzes search queries to extract intent and scope.
    
    Uses pattern matching for fast analysis, with optional
    CodeBERT-based deep analysis for complex queries.
    """
    
    _instance: Optional["QueryAnalyzer"] = None
    
    # Intent keywords
    LOOKUP_KEYWORDS = ["find", "where", "locate", "get", "show", "search"]
    EXPLANATION_KEYWORDS = ["how", "why", "explain", "what does", "understand"]
    NAVIGATION_KEYWORDS = ["definition", "defined", "declaration", "import", "from"]
    EXAMPLE_KEYWORDS = ["example", "usage", "sample", "demo", "how to use"]
    DEBUG_KEYWORDS = ["error", "bug", "fix", "exception", "crash", "fail"]
    
    # Language patterns
    LANGUAGE_PATTERNS = {
        r"\.py\b|python|def |class ": "python",
        r"\.js\b|javascript|function |const |=>": "javascript",
        r"\.ts\b|typescript|interface |type ": "typescript",
        r"\.go\b|golang|func |package ": "go",
        r"\.rs\b|rust|fn |impl |struct ": "rust",
        r"\.java\b|java|public class": "java",
        r"\.cpp\b|\.c\b|c\+\+|#include": "cpp",
    }
    
    # Path pattern
    PATH_PATTERN = re.compile(r'(?:in|from|at)\s+["\']?([^\s"\']+(?:/[^\s"\']+)+)["\']?', re.IGNORECASE)
    
    # Symbol pattern (function/class names)
    SYMBOL_PATTERN = re.compile(r'\b([A-Z][a-zA-Z0-9]*|[a-z_][a-z0-9_]*(?:_[a-z0-9_]+)+)\b')
    
    def __init__(self):
        # Stateless
        pass
    
    @classmethod
    def get_instance(cls) -> "QueryAnalyzer":
        """Get singleton instance."""
        if cls._instance is None:
            cls._instance = cls()
        return cls._instance
    
    def _get_model(self):
        """Get model tuple (model, tokenizer) from manager."""
        if not settings.QUERY_ANALYSIS_ENABLED:
            return None
            
        from src.services.model_manager import get_model_manager
        manager = get_model_manager()
        
        def loader():
            from transformers import AutoTokenizer, AutoModel
            logger.info(f"Loading query model: {settings.QUERY_MODEL}")
            tokenizer = AutoTokenizer.from_pretrained(settings.QUERY_MODEL)
            model = AutoModel.from_pretrained(settings.QUERY_MODEL)
            # Query model usually small, keep on CPU or auto
            return {"model": model, "tokenizer": tokenizer}
            
        manager.load_model("query_analyzer", loader)
        return manager.get_model_instance("query_analyzer")
    
    def analyze(self, query: str) -> QueryAnalysis:
        """
        Analyze a search query.
        
        Args:
            query: Raw search query
            
        Returns:
            QueryAnalysis with intent, filters, and processed query
        """
        if not query or not query.strip():
            return QueryAnalysis(
                original_query=query,
                processed_query=query,
                intent=QueryIntent.LOOKUP,
                confidence=0.5,
                language_hints=[],
                path_hints=[],
                symbol_hints=[],
                filters={}
            )
        
        query = query.strip()
        query_lower = query.lower()
        
        # 1. Detect intent
        intent, confidence = self._detect_intent(query_lower)
        
        # 2. Extract language hints
        language_hints = self._extract_languages(query_lower)
        
        # 3. Extract path hints
        path_hints = self._extract_paths(query)
        
        # 4. Extract symbol hints
        symbol_hints = self._extract_symbols(query)
        
        # 5. Build filters
        filters = {}
        if language_hints:
            filters["language"] = language_hints[0] if len(language_hints) == 1 else language_hints
        if path_hints:
            filters["path_prefix"] = path_hints[0]
        
        # 6. Process query (remove filter syntax)
        processed_query = self._clean_query(query)
        
        return QueryAnalysis(
            original_query=query,
            processed_query=processed_query,
            intent=intent,
            confidence=confidence,
            language_hints=language_hints,
            path_hints=path_hints,
            symbol_hints=symbol_hints,
            filters=filters
        )
    
    def _detect_intent(self, query_lower: str) -> Tuple[QueryIntent, float]:
        """Detect query intent from keywords."""
        scores = {
            QueryIntent.LOOKUP: 0.0,
            QueryIntent.EXPLANATION: 0.0,
            QueryIntent.NAVIGATION: 0.0,
            QueryIntent.EXAMPLE: 0.0,
            QueryIntent.DEBUG: 0.0,
        }
        
        for kw in self.LOOKUP_KEYWORDS:
            if kw in query_lower:
                scores[QueryIntent.LOOKUP] += 1
        
        for kw in self.EXPLANATION_KEYWORDS:
            if kw in query_lower:
                scores[QueryIntent.EXPLANATION] += 1.5
        
        for kw in self.NAVIGATION_KEYWORDS:
            if kw in query_lower:
                scores[QueryIntent.NAVIGATION] += 1.2
        
        for kw in self.EXAMPLE_KEYWORDS:
            if kw in query_lower:
                scores[QueryIntent.EXAMPLE] += 1.3
        
        for kw in self.DEBUG_KEYWORDS:
            if kw in query_lower:
                scores[QueryIntent.DEBUG] += 1.4
        
        # Default to LOOKUP if no strong signal
        max_intent = max(scores, key=scores.get)
        max_score = scores[max_intent]
        
        if max_score == 0:
            return QueryIntent.LOOKUP, 0.5
        
        # Normalize confidence
        confidence = min(0.95, 0.5 + (max_score * 0.15))
        return max_intent, confidence
    
    def _extract_languages(self, query_lower: str) -> List[str]:
        """Extract programming language hints from query."""
        languages = []
        for pattern, lang in self.LANGUAGE_PATTERNS.items():
            if re.search(pattern, query_lower):
                if lang not in languages:
                    languages.append(lang)
        return languages
    
    def _extract_paths(self, query: str) -> List[str]:
        """Extract file/directory path hints from query."""
        matches = self.PATH_PATTERN.findall(query)
        return list(set(matches))
    
    def _extract_symbols(self, query: str) -> List[str]:
        """Extract potential function/class name hints."""
        # Filter out common words
        stopwords = {"the", "and", "for", "with", "from", "this", "that", "how", "what", "where", "find"}
        matches = self.SYMBOL_PATTERN.findall(query)
        symbols = [m for m in matches if m.lower() not in stopwords and len(m) > 2]
        return list(set(symbols))[:5]  # Limit to 5
    
    def _clean_query(self, query: str) -> str:
        """Remove filter syntax from query, keep semantic content."""
        # Remove path specifications
        cleaned = self.PATH_PATTERN.sub("", query)
        # Remove "in python", "in javascript" etc
        cleaned = re.sub(r'\bin\s+(python|javascript|typescript|go|rust|java|cpp?)\b', '', cleaned, flags=re.IGNORECASE)
        # Clean up whitespace
        cleaned = re.sub(r'\s+', ' ', cleaned).strip()
        return cleaned if cleaned else query


# Module-level convenience functions

def analyze_query(query: str) -> QueryAnalysis:
    """Analyze a query using the default analyzer."""
    return QueryAnalyzer.get_instance().analyze(query)


def get_query_analyzer() -> QueryAnalyzer:
    """Get global query analyzer instance."""
    return QueryAnalyzer.get_instance()
