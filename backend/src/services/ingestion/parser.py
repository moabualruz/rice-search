# from unstructured.partition.auto import partition # Lazy
# from unstructured.documents.elements import Element
from typing import List

class DocumentParser:
    @staticmethod
    def parse_file(file_path: str) -> str:
        """
        Parses a file using Unstructured and returns the combined text content.
        """
        try:
            from unstructured.partition.auto import partition
            from unstructured.documents.elements import Element
            
            elements: List[Element] = partition(filename=file_path)
            # Combine all text elements into a single string with newlines
            text_content = "\n\n".join([str(el) for el in elements])
            return text_content
        except Exception as e:
            # Fallback for simple text files if Unstructured fails or missing dep
            if file_path.endswith((".txt", ".md", ".py", ".js", ".go")):
                with open(file_path, "r", encoding="utf-8") as f:
                    return f.read()
            raise e
