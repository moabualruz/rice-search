from src.worker import celery_app

@celery_app.task
def ingest_file(file_path: str):
    return f"Ingesting {file_path}"
