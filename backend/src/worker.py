from celery import Celery
from src.core.config import settings

celery_app = Celery(
    "rice_worker",
    broker=settings.REDIS_URL,
    backend=settings.REDIS_URL,
    include=["src.tasks.ingestion"] # Future tasks
)

celery_app.conf.update(
    task_serializer="json",
    accept_content=["json"],
    result_serializer="json",
    timezone="UTC",
    enable_utc=True,
)

# Test task
@celery_app.task
def echo_task(word: str):
    return f"Echo: {word}"
