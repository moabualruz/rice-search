"""
Prometheus metrics endpoint.

Exposes application metrics in Prometheus format.
"""

from fastapi import APIRouter
from fastapi.responses import PlainTextResponse
import time

from src.services.admin.admin_store import get_admin_store
from src.db.qdrant import get_qdrant_client
from src.core.config import settings

router = APIRouter()

# Metrics collection start time
START_TIME = time.time()


def get_metrics_text() -> str:
    """Generate Prometheus-format metrics."""
    store = get_admin_store()
    lines = []
    
    # Application info
    lines.append("# HELP rice_search_info Application information")
    lines.append("# TYPE rice_search_info gauge")
    lines.append(f'rice_search_info{{version="1.0.0",sparse_enabled="{settings.SPARSE_ENABLED}"}} 1')
    
    # Uptime
    uptime = time.time() - START_TIME
    lines.append("# HELP rice_search_uptime_seconds Application uptime in seconds")
    lines.append("# TYPE rice_search_uptime_seconds gauge")
    lines.append(f"rice_search_uptime_seconds {uptime:.2f}")
    
    # Request counters
    lines.append("# HELP rice_search_requests_total Total number of requests")
    lines.append("# TYPE rice_search_requests_total counter")
    lines.append(f"rice_search_requests_total {store.get_counter('requests_total')}")
    
    lines.append("# HELP rice_search_search_requests_total Total number of search requests")
    lines.append("# TYPE rice_search_search_requests_total counter")
    lines.append(f"rice_search_search_requests_total {store.get_counter('search_requests')}")
    
    lines.append("# HELP rice_search_ingest_requests_total Total number of ingest requests")
    lines.append("# TYPE rice_search_ingest_requests_total counter")
    lines.append(f"rice_search_ingest_requests_total {store.get_counter('ingest_requests')}")
    
    # Latency percentiles
    latencies = store.get_latency_percentiles()
    lines.append("# HELP rice_search_latency_p50_seconds P50 latency in seconds")
    lines.append("# TYPE rice_search_latency_p50_seconds gauge")
    lines.append(f"rice_search_latency_p50_seconds {latencies.get('p50', 0) / 1000:.4f}")
    
    lines.append("# HELP rice_search_latency_p95_seconds P95 latency in seconds")
    lines.append("# TYPE rice_search_latency_p95_seconds gauge")
    lines.append(f"rice_search_latency_p95_seconds {latencies.get('p95', 0) / 1000:.4f}")
    
    lines.append("# HELP rice_search_latency_p99_seconds P99 latency in seconds")
    lines.append("# TYPE rice_search_latency_p99_seconds gauge")
    lines.append(f"rice_search_latency_p99_seconds {latencies.get('p99', 0) / 1000:.4f}")
    
    # Index size (from Qdrant)
    try:
        qdrant = get_qdrant_client()
        collection_info = qdrant.get_collection("rice_chunks")
        points_count = collection_info.points_count or 0
        lines.append("# HELP rice_search_index_points_total Total points in index")
        lines.append("# TYPE rice_search_index_points_total gauge")
        lines.append(f"rice_search_index_points_total {points_count}")
    except:
        pass
    
    # System resources (from psutil via admin store)
    import psutil
    cpu = psutil.cpu_percent(interval=0.1)
    mem = psutil.virtual_memory()
    
    lines.append("# HELP rice_search_cpu_usage_percent CPU usage percentage")
    lines.append("# TYPE rice_search_cpu_usage_percent gauge")
    lines.append(f"rice_search_cpu_usage_percent {cpu:.1f}")
    
    lines.append("# HELP rice_search_memory_used_bytes Memory used in bytes")
    lines.append("# TYPE rice_search_memory_used_bytes gauge")
    lines.append(f"rice_search_memory_used_bytes {mem.used}")
    
    lines.append("# HELP rice_search_memory_total_bytes Total memory in bytes")
    lines.append("# TYPE rice_search_memory_total_bytes gauge")
    lines.append(f"rice_search_memory_total_bytes {mem.total}")
    
    return "\n".join(lines) + "\n"


@router.get("/metrics", response_class=PlainTextResponse)
async def prometheus_metrics():
    """Prometheus metrics endpoint."""
    return get_metrics_text()
