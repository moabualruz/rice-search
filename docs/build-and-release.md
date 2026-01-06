# Build & Release Guide

Build process, versioning, and release management for Rice Search.

## Table of Contents

- [Build System](#build-system)
- [Versioning](#versioning)
- [Docker Images](#docker-images)
- [Release Process](#release-process)
- [CI/CD Pipeline](#cicd-pipeline)
- [Artifact Management](#artifact-management)

---

## Build System

### Makefile Commands

Rice Search uses `Makefile` for common build tasks:

```bash
# Start all services
make up

# Stop all services
make down

# View logs
make logs

# Run tests
make test

# Run E2E tests
make e2e

# View API logs
make api-logs

# View worker logs
make worker-logs

# Install dependencies
make install

# Clean build artifacts
make clean
```

### Build Components

**1. Backend (Python)**
```bash
cd backend

# Install in development mode
pip install -e .[dev]

# Install for production
pip install .

# Freeze dependencies
pip freeze > requirements.txt
```

**2. Frontend (Node.js)**
```bash
cd frontend

# Install dependencies
npm install

# Development build
npm run dev

# Production build
npm run build

# Start production server
npm start
```

**3. Rust Tantivy Service**
```bash
cd rust-tantivy

# Build debug
cargo build

# Build release (optimized)
cargo build --release

# Run
./target/release/tantivy-server
```

**4. Docker Images**
```bash
# Build all images
docker compose -f deploy/docker-compose.yml build

# Build specific service
docker compose -f deploy/docker-compose.yml build backend-api

# Build with no cache
docker compose -f deploy/docker-compose.yml build --no-cache
```

---

## Versioning

### Semantic Versioning

Rice Search follows [Semantic Versioning](https://semver.org/):

```
MAJOR.MINOR.PATCH

Example: 1.2.3
- MAJOR: 1 (breaking changes)
- MINOR: 2 (new features, backward compatible)
- PATCH: 3 (bug fixes, backward compatible)
```

**Version Increment Rules:**
- **MAJOR**: Breaking API changes, incompatible database schema
- **MINOR**: New features, new endpoints, backward compatible
- **PATCH**: Bug fixes, performance improvements

### Version Locations

**1. Backend:**
```python
# backend/pyproject.toml
[project]
name = "rice-search"
version = "1.2.3"
```

**2. Frontend:**
```json
// frontend/package.json
{
  "name": "rice-search-frontend",
  "version": "1.2.3"
}
```

**3. Settings:**
```yaml
# backend/settings.yaml
app:
  version: "1.2.3"
```

### Version Command

```bash
# Check version via CLI
ricesearch --version
# Output: ricesearch 1.2.3

# Check via API
curl http://localhost:8000/health
# Output: {"status": "ok", "version": "1.2.3"}
```

---

## Docker Images

### Image Naming

```
<registry>/<repository>:<tag>

Examples:
- ghcr.io/youruser/rice-search-backend:1.2.3
- ghcr.io/youruser/rice-search-frontend:1.2.3
- ghcr.io/youruser/rice-search-tantivy:1.2.3
```

### Building Images

**Backend:**
```dockerfile
# backend/Dockerfile
FROM python:3.12-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

CMD ["uvicorn", "src.main:app", "--host", "0.0.0.0", "--port", "8000"]
```

```bash
# Build
docker build -t rice-search-backend:1.2.3 backend/

# Tag for registry
docker tag rice-search-backend:1.2.3 ghcr.io/youruser/rice-search-backend:1.2.3

# Push
docker push ghcr.io/youruser/rice-search-backend:1.2.3
```

**Frontend:**
```dockerfile
# frontend/Dockerfile
FROM node:18-alpine AS builder

WORKDIR /app
COPY package*.json ./
RUN npm ci

COPY . .
RUN npm run build

FROM node:18-alpine
WORKDIR /app
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/public ./public
COPY --from=builder /app/package*.json ./
RUN npm ci --only=production

CMD ["npm", "start"]
```

### Multi-Architecture Builds

```bash
# Build for multiple platforms
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t ghcr.io/youruser/rice-search-backend:1.2.3 \
  --push \
  backend/
```

---

## Release Process

### 1. Pre-Release Checklist

- [ ] All tests passing (`make test`)
- [ ] E2E tests passing (`make e2e`)
- [ ] Documentation updated
- [ ] CHANGELOG.md updated
- [ ] Version bumped in all files
- [ ] No uncommitted changes

### 2. Version Bump

```bash
# Update version in all locations
# 1. backend/pyproject.toml
# 2. frontend/package.json
# 3. backend/settings.yaml

# Example: Bump to 1.3.0
sed -i 's/version = "1.2.3"/version = "1.3.0"/' backend/pyproject.toml
sed -i 's/"version": "1.2.3"/"version": "1.3.0"/' frontend/package.json
sed -i 's/version: "1.2.3"/version: "1.3.0"/' backend/settings.yaml
```

### 3. Update CHANGELOG

```markdown
# CHANGELOG.md

## [1.3.0] - 2026-01-15

### Added
- New BM42 hybrid retrieval support
- CLI watch mode improvements

### Changed
- Updated embedding model to qwen3-embedding:4b
- Improved deduplication logic

### Fixed
- Fixed file path display in search results
- Resolved dimension mismatch errors
```

### 4. Create Git Tag

```bash
# Commit changes
git add .
git commit -m "chore: Bump version to 1.3.0"

# Create annotated tag
git tag -a v1.3.0 -m "Release v1.3.0"

# Push tag
git push origin v1.3.0
```

### 5. Build and Push Docker Images

```bash
# Build all images
docker compose -f deploy/docker-compose.yml build

# Tag with version
docker tag rice-search-backend:latest ghcr.io/youruser/rice-search-backend:1.3.0
docker tag rice-search-frontend:latest ghcr.io/youruser/rice-search-frontend:1.3.0
docker tag rice-search-tantivy:latest ghcr.io/youruser/rice-search-tantivy:1.3.0

# Tag as latest
docker tag ghcr.io/youruser/rice-search-backend:1.3.0 ghcr.io/youruser/rice-search-backend:latest

# Push all tags
docker push ghcr.io/youruser/rice-search-backend:1.3.0
docker push ghcr.io/youruser/rice-search-backend:latest
```

### 6. Create GitHub Release

```bash
# Using GitHub CLI
gh release create v1.3.0 \
  --title "v1.3.0" \
  --notes-file CHANGELOG.md \
  --latest
```

**Manual (GitHub Web UI):**
1. Go to repository → Releases → Draft new release
2. Choose tag: `v1.3.0`
3. Release title: `v1.3.0`
4. Copy relevant CHANGELOG section
5. Attach binaries (if any)
6. Publish release

---

## CI/CD Pipeline

### GitHub Actions Example

```yaml
# .github/workflows/release.yml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  build-and-push:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v2

      - name: Log in to GitHub Container Registry
        uses: docker/login-action@v2
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Extract version from tag
        id: version
        run: echo "VERSION=${GITHUB_REF#refs/tags/v}" >> $GITHUB_OUTPUT

      - name: Build and push backend
        uses: docker/build-push-action@v4
        with:
          context: ./backend
          push: true
          tags: |
            ghcr.io/${{ github.repository }}/backend:${{ steps.version.outputs.VERSION }}
            ghcr.io/${{ github.repository }}/backend:latest

      - name: Build and push frontend
        uses: docker/build-push-action@v4
        with:
          context: ./frontend
          push: true
          tags: |
            ghcr.io/${{ github.repository }}/frontend:${{ steps.version.outputs.VERSION }}
            ghcr.io/${{ github.repository }}/frontend:latest

      - name: Create GitHub Release
        uses: softprops/action-gh-release@v1
        with:
          generate_release_notes: true
```

### Continuous Integration

```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v3

      - name: Set up Python
        uses: actions/setup-python@v4
        with:
          python-version: '3.12'

      - name: Install dependencies
        run: |
          cd backend
          pip install -e .[dev]

      - name: Run tests
        run: |
          cd backend
          pytest tests/ -v

      - name: Upload coverage
        uses: codecov/codecov-action@v3
```

---

## Artifact Management

### Python Package (PyPI)

```bash
# Build distribution
cd backend
python -m build

# Upload to PyPI
python -m twine upload dist/*
```

**pyproject.toml:**
```toml
[build-system]
requires = ["setuptools>=61.0"]
build-backend = "setuptools.build_meta"

[project]
name = "rice-search"
version = "1.3.0"
description = "Hybrid search platform for code and documents"
readme = "README.md"
requires-python = ">=3.12"
```

### npm Package (Registry)

```bash
# Publish to npm
cd frontend
npm publish
```

**package.json:**
```json
{
  "name": "@yourorg/rice-search-frontend",
  "version": "1.3.0",
  "private": false,
  "publishConfig": {
    "registry": "https://npm.pkg.github.com"
  }
}
```

### Binary Releases (CLI)

```bash
# Build standalone binary (PyInstaller)
cd backend
pyinstaller --onefile --name ricesearch src/cli/ricesearch/main.py

# Upload to GitHub release
gh release upload v1.3.0 dist/ricesearch
```

---

## Summary

**Release Workflow:**

```bash
# 1. Run tests
make test && make e2e

# 2. Update version
# Edit pyproject.toml, package.json, settings.yaml

# 3. Update CHANGELOG
nano CHANGELOG.md

# 4. Commit and tag
git add .
git commit -m "chore: Release v1.3.0"
git tag -a v1.3.0 -m "Release v1.3.0"
git push origin main --tags

# 5. Build and push images
docker compose -f deploy/docker-compose.yml build
docker tag ... ghcr.io/...
docker push ghcr.io/...

# 6. Create GitHub release
gh release create v1.3.0 --generate-notes
```

**Build Commands:**
```bash
# Backend
cd backend && pip install -e .[dev]

# Frontend
cd frontend && npm install && npm run build

# Tantivy
cd rust-tantivy && cargo build --release

# Docker
docker compose -f deploy/docker-compose.yml build
```

**Versioning:**
- `MAJOR.MINOR.PATCH` (e.g., 1.3.0)
- Update in: `pyproject.toml`, `package.json`, `settings.yaml`
- Tag format: `v1.3.0`

For more details:
- [Development](development.md) - Dev workflow
- [Deployment](deployment.md) - Deploying releases
- [Testing](testing.md) - Test before release
- [Operations](operations.md) - Managing releases

---

**[Back to Documentation Index](README.md)**
