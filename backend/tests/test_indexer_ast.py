import pytest
from unittest.mock import MagicMock, patch, mock_open
import uuid
# from src.services.ingestion.indexer import Indexer
# from src.services.ingestion.ast_parser import ASTChunk

import sys
mock_unstructured = MagicMock()
sys.modules["unstructured"] = mock_unstructured
sys.modules["unstructured.partition"] = mock_unstructured
sys.modules["unstructured.partition.auto"] = mock_unstructured
sys.modules["unstructured.documents"] = mock_unstructured
sys.modules["unstructured.documents.elements"] = mock_unstructured

# Also mock tree_sitter_languages if needed
sys.modules["tree_sitter_languages"] = MagicMock()

@pytest.fixture
def Indexer():
    from src.services.ingestion.indexer import Indexer
    return Indexer


@pytest.fixture
def ASTChunk():
    from src.services.ingestion.ast_parser import ASTChunk
    return ASTChunk


@pytest.fixture
def mock_qdrant():
    return MagicMock()

@pytest.fixture
def mock_model():
    model = MagicMock()
    # Mock encode to return a fake vector
    model.encode.return_value = MagicMock(tolist=lambda: [0.1, 0.2, 0.3])
    return model

@pytest.fixture
def mock_ast_parser():
    import src.services.ingestion.indexer
    with patch("src.services.ingestion.indexer.get_ast_parser") as mock:

        parser_instance = MagicMock()
        mock.return_value = parser_instance
        yield parser_instance

def test_ingest_uses_ast_parser(mock_qdrant, mock_model, mock_ast_parser, Indexer, ASTChunk):
    """Verify that Indexer uses ASTParser instead of simple chunking."""
    indexer = Indexer(mock_qdrant, mock_model)

    
    # Mock file content read
    with patch("builtins.open", mock_open(read_data="def foo(): pass")):
        # Mock AST parser response
        mock_ast_parser.parse_file.return_value = [
            ASTChunk(
                content="def foo(): pass",
                chunk_type="function",
                language="python",
                symbols=["foo"],
                start_line=1,
                end_line=1,
                node_type="function_definition"
            )
        ]
        
        # Mock DocumentParser just in case, though we want to bypass it or use it only for raw text
        with patch("src.services.ingestion.indexer.DocumentParser.parse_file", return_value="def foo(): pass"):
            indexer.ingest_file("/tmp/test.py", "repo", "org")
            
    # Verify AST parser was called
    mock_ast_parser.parse_file.assert_called()
    
    # Verify upsert was called with AST metadata
    args, kwargs = mock_qdrant.upsert.call_args
    points = kwargs['points']
    assert len(points) == 1
    payload = points[0].payload
    assert payload["chunk_type"] == "function"
    assert payload["symbols"] == ["foo"]

def test_deterministic_chunk_ids(mock_qdrant, mock_model, mock_ast_parser, Indexer, ASTChunk):
    """Verify that identical chunks generate identical IDs."""
    indexer = Indexer(mock_qdrant, mock_model)
    
    # Setup identical chunks
    chunk_data = ASTChunk(
        content="def foo(): pass",
        chunk_type="function",
        language="python",
        symbols=["foo"],
        start_line=1,
        end_line=1,
        node_type="function_definition"
    )
    
    # We need to expose the ID generation logic or mock the internal method to test it,
    # or better, strictly check the points passed to upsert.
    
    mock_ast_parser.parse_file.return_value = [chunk_data]
    
    with patch("src.services.ingestion.indexer.DocumentParser.parse_file", return_value="def foo(): pass"):
        # Run 1
        indexer.ingest_file("/tmp/test.py", "repo", "org")
        args1, _ = mock_qdrant.upsert.call_args
        keys1 = mock_qdrant.upsert.call_args[1]['points'][0].id
        
        # Run 2
        indexer.ingest_file("/tmp/test.py", "repo", "org")
        keys2 = mock_qdrant.upsert.call_args[1]['points'][0].id
        
        assert keys1 == keys2, "Chunk ID must be deterministic"

def test_different_ids_for_changes(mock_qdrant, mock_model, mock_ast_parser, Indexer, ASTChunk):
    """Verify that different content generates different IDs."""
    indexer = Indexer(mock_qdrant, mock_model)
    
    chunk1 = ASTChunk(
        content="def foo(): pass",
        chunk_type="function",
        language="python",
        symbols=["foo"],
        start_line=1,
        end_line=1,
        node_type="function_definition"
    )
    
    chunk2 = ASTChunk(
        content="def foo(): return 1", # Changed content
        chunk_type="function",
        language="python",
        symbols=["foo"],
        start_line=1,
        end_line=1,
        node_type="function_definition"
    )
    
    # Run 1
    mock_ast_parser.parse_file.return_value = [chunk1]
    with patch("src.services.ingestion.indexer.DocumentParser.parse_file", return_value="..."):
        indexer.ingest_file("/tmp/test.py", "repo", "org")
        id1 = mock_qdrant.upsert.call_args[1]['points'][0].id
        
    # Run 2
    mock_ast_parser.parse_file.return_value = [chunk2]
    with patch("src.services.ingestion.indexer.DocumentParser.parse_file", return_value="..."):
        indexer.ingest_file("/tmp/test.py", "repo", "org")
        id2 = mock_qdrant.upsert.call_args[1]['points'][0].id
        
    assert id1 != id2, "Chunk IDs must differ for different content"
