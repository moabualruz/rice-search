from celery import Celery
from src.core.config import settings
from src.services.admin.admin_store import get_admin_store

# Read worker config from Redis
store = get_admin_store()
config = store.get_effective_config()

worker_pool = config.get("worker_pool", "threads")
worker_concurrency = config.get("worker_concurrency", 10)

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

# Explicitly Auto-discovery source
app.autodiscover_tasks(['src.tasks'])

@app.task
def echo_task(message):
    return message
