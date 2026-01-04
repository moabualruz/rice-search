import os
import re
import asyncio
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
from src.services.inference.bentoml_client import get_bentoml_client
import logging

logger = logging.getLogger(__name__)

class RAGEngine:
    def __init__(self):
        # Setup LLM - prefer BentoML, then OpenAI, then FakeListLLM
        self.bentoml_client = None
        api_key = os.getenv("OPENAI_API_KEY")
        
        # Try BentoML first
        try:
            self.bentoml_client = get_bentoml_client()
            # We assume it exists for now, health check is async so we skip it in __init__
            logger.info("RAG Engine: Using BentoML for LLM")
            self.llm = None  # Will use bentoml_client.chat() directly
        except Exception as e:
            logger.warning(f"BentoML client init failed: {e}")
            if api_key:
                self.llm = OpenAI(temperature=0)
            else:
                # Fallback for "Iron Core" without external deps
                logger.warning("No LLM available - using simulated responses")
                responses = [
                    "LLM not available. Please start BentoML service to enable RAG answers.",
                ]
                self.llm = FakeListLLM(responses=responses)

        self.retriever = Retriever()
        
        self.prompt_template = """You are an expert pair programmer. Use the following pieces of context to answer the question at the end.
Each context chunk is marked with a Source ID like [1]. 
Cite your sources in your answer using these IDs, e.g. "The function `foo` handles X [1]."

If the context is insufficient to answer the question fully, you may request ONE additional search by outputting STRICTLY:
SEARCH: <your refined search query>

Context:
{context}

Question: {question}
Answer:"""
        
        self.prompt = PromptTemplate(
            input_variables=["context", "question"],
            template=self.prompt_template
        )
        
        if self.llm:
            self.chain = self.prompt | self.llm | StrOutputParser()
        else:
            self.chain = None  # Using BentoML directly

    def format_context(self, docs: List[Dict]) -> str:
        """
        Formats documents with citation markers and metadata.
        Output format:
        Source [1]: path/to/file.py (Lines 10-20)
        ...content...
        """
        formatted = []
        for i, doc in enumerate(docs, 1):
            # Handle both nested metadata and flat payload structure
            meta = doc.get("metadata", {})
            path = doc.get("file_path") or meta.get("file_path", "unknown")
            start = doc.get("start_line") or meta.get("start_line", "?")
            end = doc.get("end_line") or meta.get("end_line", "?")
            
            header = f"\nSource [{i}]: {path} (Lines {start}-{end})"
            content = doc.get("text", "").strip()
            formatted.append(f"{header}\n{content}")
            
        return "\n".join(formatted)
    
    async def _generate_response(self, context: str, question: str) -> str:
        """Generate response using BentoML or LangChain LLM."""
        prompt_text = self.prompt_template.format(context=context, question=question)
        
        # Try BentoML first
        if self.bentoml_client:
            try:
                # Async chat call
                response = await self.bentoml_client.chat(
                    messages=[{"role": "user", "content": prompt_text}],
                    max_tokens=1024,
                    temperature=0.1
                )
                return response
            except Exception as e:
                logger.warning(f"BentoML chat failed: {e}")
                if self.chain:
                    # Fallback to LangChain (run in thread if blocking)
                    # Try ainvoke if available (modern langchain), else to_thread
                    if hasattr(self.chain, "ainvoke"):
                         return await self.chain.ainvoke({"context": context, "question": question})
                    else:
                         return await asyncio.to_thread(self.chain.invoke, {"context": context, "question": question})
                return f"LLM unavailable. Error: {e}"
        
        # Use LangChain chain
        if self.chain:
             if hasattr(self.chain, "ainvoke"):
                  return await self.chain.ainvoke({"context": context, "question": question})
             else:
                  return await asyncio.to_thread(self.chain.invoke, {"context": context, "question": question})
        
        return "No LLM available. Please start BentoML service."

    async def ask(self, query: str, org_id: str = "public") -> Dict:
        """
        Standard single-step RAG.
        """
        return await self.ask_with_deep_dive(query, org_id=org_id, max_steps=1)

    async def ask_with_deep_dive(self, query: str, org_id: str = "public", max_steps: int = 3) -> Dict:
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
                # Call async retriever
                new_docs = await self.retriever.search(current_query, limit=3, org_id=org_id)
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
            
            # 3. Generate - use BentoML or LangChain chain (async)
            response = await self._generate_response(context_text, query)
            
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
             # response might be undefined if loop didn't run, but max_steps > 0
            answer = re.sub(r"SEARCH:.*", "*Max search steps reached. Best effort answer:*", response).strip()

        return {
            "answer": answer,
            "sources": all_docs,
            "steps_taken": steps_taken
        }

