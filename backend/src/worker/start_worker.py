#!/usr/bin/env python
"""
Celery worker entry point with dynamic configuration from Redis.
Reads pool type and concurrency from AdminStore at startup.
"""
import os
import sys

# Ensure src is on path
sys.path.insert(0, '/app')

from celery import Celery
from src.core.config import settings
from src.services.admin.admin_store import get_admin_store

# Read worker config from Redis
store = get_admin_store()
config = store.get_effective_config()

worker_pool = config.get("worker_pool", "threads")
worker_concurrency = config.get("worker_concurrency", 10)

print(f"Starting Celery worker with pool={worker_pool}, concurrency={worker_concurrency}")

# Create Celery app
app = Celery(
    "rice_worker",
    broker=settings.REDIS_URL,
    backend=settings.REDIS_URL
)

app.conf.update(
    task_serializer='json',
    accept_content=['json'],
    result_serializer='json',
    timezone='UTC',
    enable_utc=True,
    worker_pool=worker_pool,
    worker_concurrency=worker_concurrency
)

# Auto-discover tasks
app.autodiscover_tasks(['src.worker'])

if __name__ == '__main__':
    # Start worker with config from Redis
    app.start(argv=[
        'worker',
        f'--pool={worker_pool}',
        f'--concurrency={worker_concurrency}',
        '--loglevel=info'
    ])
