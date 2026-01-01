from locust import HttpUser, task, between, constant

class StandardUser(HttpUser):
    wait_time = between(1, 3)

    @task(3)
    def search_query(self):
        """Perform a standard search query"""
        # Note: Requires data in Qdrant for meaningful results, but endpoint should return 200 or 404
        self.client.post("/api/v1/search", json={"query": "test", "mode": "hybrid"}, name="/search")

    @task(1)
    def health_check(self):
        """Check API health"""
        self.client.get("/api/v1/metrics", name="/metrics")
