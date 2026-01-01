"""
Unit tests for DocumentParser.
"""
import pytest
from unittest.mock import patch, MagicMock, mock_open
from src.services.ingestion.parser import DocumentParser

def test_parse_file_unstructured_success():
    """Test successful parsing via unstructured."""
    with patch("src.services.ingestion.parser.partition") as mock_partition:
        mock_el = MagicMock()
        mock_el.__str__.return_value = "parsed content"
        mock_partition.return_value = [mock_el]
        
        result = DocumentParser.parse_file("dummy.pdf")
        assert result == "parsed content"

def test_parse_file_fallback_text():
    """Test fallback to basic file read for code/text files."""
    m_open = mock_open(read_data="raw text")
    
    with patch("src.services.ingestion.parser.partition", side_effect=Exception("Unstructured missing")):
        with patch("builtins.open", m_open):
            result = DocumentParser.parse_file("script.py")
            assert result == "raw text"

def test_parse_failure():
    """Test failure when both fail."""
    with patch("src.services.ingestion.parser.partition", side_effect=Exception("Fatal error")):
         with pytest.raises(Exception) as excinfo:
             DocumentParser.parse_file("unknown.bin")
         assert "Fatal error" in str(excinfo.value)
