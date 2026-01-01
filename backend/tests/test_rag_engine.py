

import unittest
from unittest.mock import MagicMock, patch
# Adjust import path if needed, assuming running from backend root
from src.services.rag.engine import RAGEngine

class TestRAGEngine(unittest.TestCase):

    def test_format_context(self):
        engine = RAGEngine()
        docs = [
            {
                "text": "def foo(): pass",
                "metadata": {"file_path": "src/foo.py", "start_line": 10, "end_line": 12}
            },
            {
                "text": "class Bar: pass", 
                # Missing metadata case
                "metadata": {}
            }
        ]
        
        formatted = engine.format_context(docs)
        
        self.assertIn("Source [1]: src/foo.py (Lines 10-12)", formatted)
        self.assertIn("def foo(): pass", formatted)
        self.assertIn("Source [2]: unknown (Lines ?-?)", formatted)
        self.assertIn("class Bar: pass", formatted)

    @patch("src.services.rag.engine.Retriever")
    @patch("src.services.rag.engine.OpenAI")
    def test_deep_dive_logic(self, MockOpenAI, MockRetriever):
        # Setup
        engine = RAGEngine()
        
        # Mock Retriever
        mock_retriever = MockRetriever.return_value
        mock_retriever.search.side_effect = [
            [{"text": "Doc 1", "metadata": {"file_path": "a.py"}}],  # First search
            [{"text": "Doc 2", "metadata": {"file_path": "b.py"}}]   # Second search (Deep Dive)
        ]
        
        # Mock Chain (LLM + Parser)
        mock_chain = MagicMock()
        mock_chain.invoke.side_effect = [
            "SEARCH: detailed info about X", # First response: Request search
            "Final answer based on Doc 1 and Doc 2 [1] [2]" # Second response: Final answer
        ]
        engine.chain = mock_chain
        
        # Execute
        result = engine.ask_with_deep_dive("initial query", max_steps=3)
        
        # Verify
        self.assertEqual(result["steps_taken"], 2)
        self.assertIn("Final answer", result["answer"])
        self.assertEqual(len(result["sources"]), 2) # Should have accumulated both docs
        self.assertEqual(result["sources"][0]["text"], "Doc 1")
        self.assertEqual(result["sources"][1]["text"], "Doc 2")
        
        # Verify search calls
        self.assertEqual(mock_retriever.search.call_count, 2)
        mock_retriever.search.assert_any_call("initial query", limit=3, org_id="public")
        mock_retriever.search.assert_any_call("detailed info about X", limit=3, org_id="public")

    @patch("src.services.rag.engine.Retriever")
    @patch("src.services.rag.engine.OpenAI")
    def test_deep_dive_max_steps(self, MockOpenAI, MockRetriever):
        # Setup
        engine = RAGEngine()
        
        # Mock Retriever
        mock_retriever = MockRetriever.return_value
        mock_retriever.search.return_value = [{"text": "Doc 1", "metadata": {}}]
        
        # Mock Chain to always request search
        mock_chain = MagicMock()
        mock_chain.invoke.return_value = "SEARCH: infinite loop"
        engine.chain = mock_chain
        
        # Execute with max_steps=2
        result = engine.ask_with_deep_dive("query", max_steps=2)
        
        # Verify
        self.assertEqual(result["steps_taken"], 2)
        # Should fallback to the last response but formatted
        self.assertIn("Best effort answer", result["answer"])

if __name__ == '__main__':
    unittest.main()

