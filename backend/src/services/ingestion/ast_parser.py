"""
AST-aware parser using Tree-sitter.

Parses code files into structured chunks at function/class/method boundaries.
"""

import logging
from pathlib import Path
from typing import List, Dict, Any, Optional, Tuple
from dataclasses import dataclass

# try:
#     import tree_sitter_languages
#     TREE_SITTER_AVAILABLE = True
# except ImportError:
#     TREE_SITTER_AVAILABLE = False
TREE_SITTER_AVAILABLE = None # Checked lazily

from src.core.config import settings

logger = logging.getLogger(__name__)


# Language extension mapping
EXTENSION_TO_LANGUAGE = {
    ".py": "python",
    ".js": "javascript",
    ".jsx": "javascript",
    ".ts": "typescript",
    ".tsx": "tsx",
    ".go": "go",
    ".rs": "rust",
    ".java": "java",
    ".cpp": "cpp",
    ".hpp": "cpp",
    ".cc": "cpp",
    ".c": "c",
    ".h": "c",
    ".rb": "ruby",
    ".php": "php",
}

# Node types that represent code blocks we want to extract
CODE_BLOCK_TYPES = {
    "python": ["function_definition", "class_definition"],
    "javascript": ["function_declaration", "class_declaration", "arrow_function", "method_definition"],
    "typescript": ["function_declaration", "class_declaration", "arrow_function", "method_definition"],
    "tsx": ["function_declaration", "class_declaration", "arrow_function", "method_definition"],
    "go": ["function_declaration", "method_declaration", "type_declaration"],
    "rust": ["function_item", "impl_item", "struct_item"],
    "java": ["method_declaration", "class_declaration", "interface_declaration"],
    "cpp": ["function_definition", "class_specifier"],
    "c": ["function_definition", "struct_specifier"],
    "ruby": ["method", "class"],
    "php": ["function_definition", "class_declaration", "method_declaration"],
}


@dataclass
class ASTChunk:
    """Represents a parsed code chunk with metadata."""
    content: str
    chunk_type: str  # function, class, method, module
    language: str
    symbols: List[str]
    start_line: int
    end_line: int
    node_type: str


class ASTParser:
    """
    Tree-sitter based AST parser for code-aware chunking.
    """
    
    def __init__(self):
        self.enabled = getattr(settings, 'AST_PARSING_ENABLED', True)
        self._parsers = {}
    
    def _get_parser(self, language: str):
        """Get or create parser for a language."""
        global TREE_SITTER_AVAILABLE
        if TREE_SITTER_AVAILABLE is None:
            try:
                import tree_sitter_languages
                TREE_SITTER_AVAILABLE = True
            except ImportError:
                TREE_SITTER_AVAILABLE = False
                
        if not TREE_SITTER_AVAILABLE:
            return None
        
        if language not in self._parsers:
            try:
                self._parsers[language] = tree_sitter_languages.get_parser(language)
            except Exception as e:
                logger.warning(f"Failed to load parser for {language}: {e}")
                return None
        
        return self._parsers[language]
    
    def detect_language(self, file_path: Path) -> Optional[str]:
        """Detect language from file extension."""
        ext = file_path.suffix.lower()
        return EXTENSION_TO_LANGUAGE.get(ext)
    
    def can_parse(self, file_path: Path) -> bool:
        """Check if we can parse this file type."""
        if not self.enabled:
            return False
        
        language = self.detect_language(file_path)
        return language is not None and self._get_parser(language) is not None
    
    def parse_file(self, file_path: Path, content: Optional[str] = None) -> List[ASTChunk]:
        """
        Parse a file into AST chunks.
        
        Args:
            file_path: Path to file
            content: Optional file content (if already read)
            
        Returns:
            List of ASTChunk objects
        """
        language = self.detect_language(file_path)
        if not language:
            return []
        
        parser = self._get_parser(language)
        if not parser:
            return []
        
        # Read content if not provided
        if content is None:
            try:
                with open(file_path, 'r', encoding='utf-8') as f:
                    content = f.read()
            except Exception as e:
                logger.error(f"Failed to read file {file_path}: {e}")
                return []
        
        # Parse
        try:
            tree = parser.parse(content.encode('utf-8'))
        except Exception as e:
            logger.error(f"Failed to parse {file_path}: {e}")
            return []
        
        # Extract chunks
        chunks = self._extract_chunks(tree.root_node, content, language)
        
        # If no chunks found, treat entire file as a module chunk
        if not chunks:
            lines = content.split('\n')
            chunks.append(ASTChunk(
                content=content,
                chunk_type="module",
                language=language,
                symbols=[],
                start_line=1,
                end_line=len(lines),
                node_type="module"
            ))
        
        return chunks
    
    def _extract_chunks(
        self,
        node,
        source: str,
        language: str
    ) -> List[ASTChunk]:
        """Extract code chunks from AST node."""
        chunks = []
        block_types = CODE_BLOCK_TYPES.get(language, [])
        
        def visit(node, parent_class=None):
            node_type = node.type
            
            if node_type in block_types:
                chunk = self._node_to_chunk(node, source, language, parent_class)
                if chunk:
                    chunks.append(chunk)
                    
                    # Track class name for method extraction
                    if "class" in node_type.lower():
                        class_name = self._get_node_name(node, language)
                        for child in node.children:
                            visit(child, parent_class=class_name)
                        return
            
            # Recurse
            for child in node.children:
                visit(child, parent_class)
        
        visit(node)
        return chunks
    
    def _node_to_chunk(
        self,
        node,
        source: str,
        language: str,
        parent_class: Optional[str] = None
    ) -> Optional[ASTChunk]:
        """Convert AST node to ASTChunk."""
        try:
            # Extract text
            start_byte = node.start_byte
            end_byte = node.end_byte
            content = source[start_byte:end_byte]
            
            # Get line numbers (1-indexed)
            start_line = node.start_point[0] + 1
            end_line = node.end_point[0] + 1
            
            # Determine chunk type
            node_type = node.type.lower()
            if "class" in node_type:
                chunk_type = "class"
            elif "method" in node_type or (parent_class and "function" in node_type):
                chunk_type = "method"
            else:
                chunk_type = "function"
            
            # Extract symbol name
            name = self._get_node_name(node, language)
            symbols = [name] if name else []
            
            # For methods, include class prefix
            if parent_class and name:
                symbols = [f"{parent_class}.{name}"]
            
            return ASTChunk(
                content=content,
                chunk_type=chunk_type,
                language=language,
                symbols=symbols,
                start_line=start_line,
                end_line=end_line,
                node_type=node.type
            )
        except Exception as e:
            logger.error(f"Failed to convert node to chunk: {e}")
            return None
    
    def _get_node_name(self, node, language: str) -> Optional[str]:
        """Extract the name identifier from a node."""
        # Look for name/identifier child node
        for child in node.children:
            if child.type in ["identifier", "name", "property_identifier"]:
                return child.text.decode('utf-8') if child.text else None
        return None


# Singleton instance
_parser: Optional[ASTParser] = None

def get_ast_parser() -> ASTParser:
    """Get global AST parser instance."""
    global _parser
    if _parser is None:
        _parser = ASTParser()
    return _parser
