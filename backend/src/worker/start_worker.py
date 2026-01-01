#!/usr/bin/env python
"""
Celery worker entry point with dynamic configuration from Redis.
Reads pool type and concurrency from AdminStore at startup.
"""
import os
import sys

# Ensure src is on path
# Ensure src is on path
sys.path.insert(0, '/app')

from src.worker.celery_app import app, worker_pool, worker_concurrency

# IMPORT TASKS TO REGISTER THEM
import src.tasks.ingestion

if __name__ == '__main__':
    print(f"Starting Celery worker with pool={worker_pool}, concurrency={worker_concurrency}")
    # Start worker with config from Redis
    app.start(argv=[
        'worker',
        f'--pool={worker_pool}',
        f'--concurrency={worker_concurrency}',
        '--loglevel=info'
    ])
