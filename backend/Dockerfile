FROM python:3.12-slim

WORKDIR /app

# Install system dependencies (for unstructured, etc)
RUN apt-get update && apt-get install -y \
    build-essential \
    libmagic-dev \
    && rm -rf /var/lib/apt/lists/*

COPY pyproject.toml .
COPY . .
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Default command (overridden in docker-compose)
ENTRYPOINT ["/entrypoint.sh"]
CMD ["uvicorn", "src.main:app", "--host", "0.0.0.0", "--port", "8000"]
