FROM python:3.12-slim

WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Install Python packages
RUN pip install --no-cache-dir \
    flask==3.1.0 \
    sglang[all]==0.3.7.post1 \
    torch==2.5.1 \
    transformers==4.47.1

# Copy server script
COPY server.py /app/server.py

# Expose port
EXPOSE 30002

# Run server
CMD ["python", "server.py"]
