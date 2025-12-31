const API_BASE = "http://localhost:8000/api/v1";

export type SearchResult = {
  score: number;
  text: string;
  metadata: Record<string, any>;
};

export type SearchResponse = {
  answer?: string;
  sources?: SearchResult[];
  results?: SearchResult[];
};

export const api = {
  health: async () => {
    try {
      const res = await fetch("http://localhost:8000/health");
      if (!res.ok) return false;
      const data = await res.json();
      return data.status === "healthy";
    } catch (e) {
      console.error(e);
      return false;
    }
  },

  ingest: async (file: File) => {
    const formData = new FormData();
    formData.append("file", file);

    const res = await fetch(`${API_BASE}/ingest/file`, {
      method: "POST",
      body: formData,
    });

    if (!res.ok) {
      throw new Error(`Ingestion failed: ${res.statusText}`);
    }
    return res.json();
  },

  search: async (
    query: string,
    mode: "search" | "rag" = "search"
  ): Promise<SearchResponse> => {
    const res = await fetch(`${API_BASE}/search/query`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ query, mode }),
    });

    if (!res.ok) {
      throw new Error(`Search failed: ${res.statusText}`);
    }
    return res.json();
  },
};
