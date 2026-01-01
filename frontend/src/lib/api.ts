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

  ingest: async (file: File, token?: string) => {
    const formData = new FormData();
    formData.append("file", file);

    const headers: HeadersInit = {};
    if (token) {
      headers["Authorization"] = `Bearer ${token}`;
    }

    const res = await fetch(`${API_BASE}/ingest/file`, {
      method: "POST",
      body: formData,
      headers: token ? headers : undefined,
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

  listFiles: async (
    pattern?: string,
    orgId?: string
  ): Promise<{ files: string[]; count: number }> => {
    const url = new URL(`${API_BASE}/files/list`);
    if (pattern) url.searchParams.append("pattern", pattern);
    if (orgId) url.searchParams.append("org_id", orgId);

    const res = await fetch(url.toString());
    if (!res.ok) throw new Error("Failed to list files");
    return res.json();
  },

  getFileContent: async (
    path: string
  ): Promise<{ path: string; content: string; language: string }> => {
    const url = new URL(`${API_BASE}/files/content`);
    url.searchParams.append("path", path);

    const res = await fetch(url.toString());
    if (!res.ok) throw new Error("Failed to get file content");
    return res.json();
  },

  // Stores (Phase 16 P2)
  listStores: async (): Promise<any[]> => {
    const res = await fetch(`${API_BASE}/stores/`);
    if (!res.ok) throw new Error("Failed to list stores");
    return res.json();
  },

  createStore: async (data: any): Promise<any> => {
    const res = await fetch(`${API_BASE}/stores/`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data),
    });
    if (!res.ok) throw new Error("Failed to create store");
    return res.json();
  },

  getStore: async (id: string): Promise<any> => {
    const res = await fetch(`${API_BASE}/stores/${id}`);
    if (!res.ok) throw new Error("Failed to get store");
    return res.json();
  },

  deleteStore: async (id: string): Promise<any> => {
    const res = await fetch(`${API_BASE}/stores/${id}`, {
      method: "DELETE",
    });
    if (!res.ok) throw new Error("Failed to delete store");
    return res.json();
  },
};
