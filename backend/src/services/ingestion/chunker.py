from langchain_text_splitters import RecursiveCharacterTextSplitter
from typing import List, Dict

class DocumentChunker:
    def __init__(self, chunk_size: int = 1000, chunk_overlap: int = 200):
        self.splitter = RecursiveCharacterTextSplitter(
            chunk_size=chunk_size,
            chunk_overlap=chunk_overlap,
            separators=["\n\n", "\n", " ", ""]
        )

    def chunk_text(self, text: str, metadata: Dict) -> List[Dict]:
        """
        Splits text into chunks and attaches metadata to each.
        Returns list of payload dicts for Qdrant.
        """
        docs = self.splitter.create_documents([text], metadatas=[metadata])
        
        chunks = []
        for i, doc in enumerate(docs):
            chunks.append({
                "content": doc.page_content,
                "metadata": doc.metadata,
                "chunk_index": i
            })
        return chunks

def chunk_text(text: str, chunk_size: int = 1000) -> List[str]:
    """
    Standalone function to chunk text (wrapper for compatibility).
    """
    splitter = DocumentChunker(chunk_size=chunk_size, chunk_overlap=0)
    chunks = splitter.chunk_text(text, {})
    return [c["content"] for c in chunks]
