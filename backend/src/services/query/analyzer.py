"""
Query Analyzer Service.
Parses natural language queries to extract intent and scope filters.
"""
import re
import logging
from typing import Dict, Any, Optional

logger = logging.getLogger(__name__)

class QueryAnalyzer:
    """Analyzes search queries for intent and scope."""
    
    # Intent keywords
    EXPLANATION_KEYWORDS = r"\b(explain|how|why|what is|describe)\b"
    
    # Language keywords
    LANGUAGE_PATTERN = r"\b(python|javascript|typescript|js|ts|py|go|rust|java|cpp|c\+\+|ruby|php)\b"
    
    # Scope pattern (in [language])
    SCOPE_PATTERN = r"\b(in|using|with)\s+(?P<lang>" + LANGUAGE_PATTERN + r")\b"
    
    def __init__(self):
        """Initialize query analyzer."""
        pass
        
    def analyze(self, query: str) -> Dict[str, Any]:
        """
        Analyze a query to determine intent and extraction scope variables.
        
        Args:
            query: Natural language query string
            
        Returns:
            Dict containing:
            - intent: 'lookup', 'explanation', or 'navigation'
            - scope: Dict of extracted filters (language, path, etc.)
            - refined_query: The query text with artifacts removed
        """
        if not query:
            return {"intent": "lookup", "scope": {}, "refined_query": ""}
            
        intent = self._detect_intent(query)
        scope, refined_query = self._extract_scope(query)
        
        result = {
            "intent": intent,
            "scope": scope,
            "refined_query": refined_query.strip()
        }
        
        logger.debug(f"Analyzed query: '{query}' -> {result}")
        return result

    def _detect_intent(self, query: str) -> str:
        """Detect query intent based on keywords."""
        if re.search(self.EXPLANATION_KEYWORDS, query, re.IGNORECASE):
            return "explanation"
        # Can add navigation logic here
        return "lookup"

    def _extract_scope(self, query: str) -> tuple[Dict[str, str], str]:
        """Extract scope hints and return clean query."""
        scope = {}
        clean_query = query
        
        # Extract language explicitly mentioned
        lang_match = re.search(self.LANGUAGE_PATTERN, query, re.IGNORECASE)
        if lang_match:
            # Map common aliases
            lang = lang_match.group(1).lower()
            lang_map = {"js": "javascript", "ts": "typescript", "py": "python", "c++": "cpp"}
            scope["language"] = lang_map.get(lang, lang)
            
            # Remove "in python" phrase if exists, or just keep it?
            # Test expects refined query. Let's remove the specific "in python" phrase if matched
            scope_phrase_match = re.search(self.SCOPE_PATTERN, query, re.IGNORECASE)
            if scope_phrase_match:
                clean_query = re.sub(self.SCOPE_PATTERN, "", clean_query, flags=re.IGNORECASE)
            
        return scope, clean_query
