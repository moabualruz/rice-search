import os
import re
from typing import List, Dict, Tuple
from langchain_core.prompts import PromptTemplate
from langchain_core.output_parsers import StrOutputParser
try:
    from langchain_community.llms import FakeListLLM, OpenAI
except ImportError:
    # Handle older versions or missing extras if needed, but aim for community
    from langchain.llms.fake import FakeListLLM
    from langchain.llms import OpenAI

from src.services.search.retriever import Retriever

class RAGEngine:
    def __init__(self):
        # Setup LLM
        api_key = os.getenv("OPENAI_API_KEY")
        if api_key:
            self.llm = OpenAI(temperature=0)
        else:
            # Fallback for "Iron Core" without external deps
            responses = [
                "This is a simulated RAG response based on the retrieved context.",
                "SEARCH: query optimization techniques" # Simulator for deep dive testing
            ]
            self.llm = FakeListLLM(responses=responses)

        self.retriever = Retriever()
        
        self.prompt = PromptTemplate(
            input_variables=["context", "question"],
            template="""You are an expert pair programmer. Use the following pieces of context to answer the question at the end.
Each context chunk is marked with a Source ID like [1]. 
Cite your sources in your answer using these IDs, e.g. "The function `foo` handles X [1]."

If the context is insufficient to answer the question fully, you may request ONE additional search by outputting STRICTLY:
SEARCH: <your refined search query>

Context:
{context}

Question: {question}
Answer:"""
        )
        
        self.chain = self.prompt | self.llm | StrOutputParser()

    def format_context(self, docs: List[Dict]) -> str:
        """
        Formats documents with citation markers and metadata.
        Output format:
        Source [1]: path/to/file.py (Lines 10-20)
        ...content...
        """
        formatted = []
        for i, doc in enumerate(docs, 1):
            meta = doc.get("metadata", {})
            path = meta.get("file_path", "unknown")
            start = meta.get("start_line", "?")
            end = meta.get("end_line", "?")
            
            header = f"\nSource [{i}]: {path} (Lines {start}-{end})"
            content = doc.get("text", "").strip()
            formatted.append(f"{header}\n{content}")
            
        return "\n".join(formatted)

    def ask(self, query: str, org_id: str = "public") -> Dict:
        """
        Standard single-step RAG.
        """
        return self.ask_with_deep_dive(query, org_id=org_id, max_steps=1)

    def ask_with_deep_dive(self, query: str, org_id: str = "public", max_steps: int = 3) -> Dict:
        """
        Iterative RAG pipeline that allows the model to request follow-up searches.
        """
        current_query = query
        all_docs = []
        answer = ""
        steps_taken = 0
        
        visited_queries = set()

        for step in range(max_steps):
            steps_taken += 1
            
            # 1. Retrieve
            if current_query not in visited_queries:
                new_docs = self.retriever.search(current_query, limit=3, org_id=org_id)
                visited_queries.add(current_query)
                
                # Deduplicate docs based on content or ID if available, here simple append
                # In robust impl, check IDs. For now, just append unique texts.
                existing_texts = {d["text"] for d in all_docs}
                for d in new_docs:
                    if d["text"] not in existing_texts:
                        all_docs.append(d)

            if not all_docs and step == 0:
                return {
                    "answer": "I couldn't find any relevant documents to answer your question.",
                    "sources": []
                }

            # 2. Format Context
            context_text = self.format_context(all_docs)
            
            # 3. Generate
            response = self.chain.invoke({"context": context_text, "question": query})
            
            # 4. Check for Search Request
            search_match = re.search(r"SEARCH:\s*(.+)", response)
            if search_match:
                # LLM requested a deep dive
                refined_query = search_match.group(1).strip()
                current_query = refined_query
                # Continue loop to next step
                continue
            else:
                # Final answer
                answer = response
                break
        
        # If loop finishes without break (max steps reached), use last response, stripping SEARCH command if present
        if not answer:
             # Fallback if we ended on a search command
            answer = re.sub(r"SEARCH:.*", "*Max search steps reached. Best effort answer:*", response).strip()

        return {
            "answer": answer,
            "sources": all_docs,
            "steps_taken": steps_taken
        }

