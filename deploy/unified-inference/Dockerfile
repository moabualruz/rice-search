# Use NVIDIA CUDA base image with Ubuntu 24.04 (Python 3.12 included by default)
FROM nvidia/cuda:12.6.1-cudnn-devel-ubuntu24.04

# Install system dependencies (Python 3.12 is in default repos)
RUN apt-get update && apt-get install -y \
    python3 \
    python3-dev \
    python3-pip \
    curl \
    git \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Set Python 3 as default python
RUN update-alternatives --install /usr/bin/python python /usr/bin/python3 1

# Upgrade pip (--break-system-packages is safe in Docker containers)
RUN python3 -m pip install --no-cache-dir --break-system-packages --ignore-installed --upgrade pip setuptools wheel

# Set working directory
WORKDIR /app

# Create directories
RUN mkdir -p /app/config /app/logs

# Copy entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Set environment variables
ENV PYTHONUNBUFFERED=1
ENV PYTHONPATH=/app
ENV PIP_BREAK_SYSTEM_PACKAGES=1

# Expose router port and SGLang backend ports
EXPOSE 3001
EXPOSE 30001-30020

# Health check
HEALTHCHECK --interval=30s --timeout=10s --retries=3 --start-period=60s \
    CMD curl -f http://localhost:3001/health || exit 1

# Run via entrypoint
ENTRYPOINT ["/entrypoint.sh"]
CMD ["python3", "main.py"]
