import os
from typing import List, Dict
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
            responses = ["This is a simulated RAG response based on the retrieved context."]
            self.llm = FakeListLLM(responses=responses)

        self.retriever = Retriever()
        
        self.prompt = PromptTemplate(
            input_variables=["context", "question"],
            template="""Use the following pieces of context to answer the question at the end. 
            If you don't know the answer, just say that you don't know, don't try to make up an answer.

            Context:
            {context}

            Question: {question}
            Answer:"""
        )
        
        self.chain = self.prompt | self.llm | StrOutputParser()

    def ask(self, query: str) -> Dict:
        """
        End-to-end RAG pipeline.
        """
        # 1. Retrieve
        docs = self.retriever.search(query, limit=3)
        
        if not docs:
            return {
                "answer": "I couldn't find any relevant documents to answer your question.",
                "sources": []
            }

        # 2. Format Context
        context_text = "\n\n".join([d["text"] for d in docs])
        
        # 3. Generate
        answer = self.chain.invoke({"context": context_text, "question": query})
        
        return {
            "answer": answer,
            "sources": docs
        }
