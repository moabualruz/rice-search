FROM python:3.11-slim

WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Upgrade pip
RUN pip install --upgrade pip

# Copy application files
COPY requirements.txt main.py entrypoint.sh ./
RUN chmod +x entrypoint.sh

# Expose port
EXPOSE 80

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=180s --retries=3 \
    CMD curl -f http://localhost:80/health || exit 1

# Install deps at runtime (uses mounted pip cache), then start server
ENTRYPOINT ["/app/entrypoint.sh"]
