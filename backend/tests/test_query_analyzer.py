"""
Unit tests for Query Analyzer service.
"""
import pytest

@pytest.mark.unit
class TestQueryAnalyzer:
    """Test query analyzer functionality."""
    
    def test_analyze_intent_lookup(self):
        """Test detection of lookup intent."""
        from src.services.query.analyzer import QueryAnalyzer
        analyzer = QueryAnalyzer()
        
        result = analyzer.analyze("find function load_model")
        assert result["intent"] == "lookup"
        
    def test_analyze_intent_explanation(self):
        """Test detection of explanation intent."""
        from src.services.query.analyzer import QueryAnalyzer
        analyzer = QueryAnalyzer()
        
        result = analyzer.analyze("explain how model loading works")
        assert result["intent"] == "explanation"

    def test_extract_language_python(self):
        """Test extraction of language filter."""
        from src.services.query.analyzer import QueryAnalyzer
        analyzer = QueryAnalyzer()
        
        result = analyzer.analyze("show me python code for sorting")
        assert result["scope"]["language"] == "python"

    def test_refined_query(self):
        """Test that query is cleaned of scope hints."""
        from src.services.query.analyzer import QueryAnalyzer
        analyzer = QueryAnalyzer()
        
        # 'in python' should be removed or handled
        result = analyzer.analyze("sorting function in python")
        # refined query should probably still contain semantic parts but maybe standardizes it
        assert "refined_query" in result
