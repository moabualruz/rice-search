# Rice Search Documentation

Welcome to the Rice Search documentation! This guide will help you get started, deploy, and develop with Rice Search.

## 📚 Table of Contents

### Getting Started
- **[Getting Started Guide](getting-started.md)** - Installation, first run, and basic usage
- **[Quick Reference](#quick-reference)** - Common commands at a glance

### Core Documentation
- **[Architecture](architecture.md)** - System design, components, and data flow
- **[Configuration](configuration.md)** - Settings reference and environment variables
- **[CLI Guide](cli.md)** - Command-line interface usage
- **[API Reference](api.md)** - REST API endpoints and examples

### Development & Operations
- **[Development Guide](development.md)** - Dev workflow, testing, and debugging
- **[Testing Guide](testing.md)** - Unit, integration, and E2E testing
- **[Deployment Guide](deployment.md)** - Production deployment and scaling
- **[Build & Release](build-and-release.md)** - Build process and versioning
- **[Operations Guide](operations.md)** - Monitoring, logs, and health checks

### Help & Security
- **[Troubleshooting](troubleshooting.md)** - Common issues and solutions
- **[Security Guide](security.md)** - Authentication, authorization, and best practices

---

## 🚀 Quick Reference

### Starting Rice Search

```bash
# Start all services
make up

# View logs
make logs

# Stop services
make down
```

### Using the CLI

```bash
# Index files
ricesearch index ./backend

# Watch for changes
ricesearch watch ./src --org-id myproject

# Search
ricesearch search "authentication" --limit 10
```

### API Endpoints

```bash
# Search
curl -X POST http://localhost:8000/api/v1/search/query \
  -H "Content-Type: application/json" \
  -d '{"query": "user auth", "limit": 5}'

# Health check
curl http://localhost:8000/health
```

### Service URLs

- **Frontend**: http://localhost:3000
- **API**: http://localhost:8000
- **API Docs**: http://localhost:8000/docs
- **MinIO Console**: http://localhost:9001

---

## 📖 Document Guide

### For New Users
Start here to get Rice Search running:
1. [Getting Started](getting-started.md) - Installation and first search
2. [CLI Guide](cli.md) - Learn the command-line interface
3. [Configuration](configuration.md) - Customize settings

### For Developers
Contributing to Rice Search:
1. [Development Guide](development.md) - Set up your dev environment
2. [Architecture](architecture.md) - Understand the system design
3. [Testing Guide](testing.md) - Run and write tests
4. [API Reference](api.md) - Explore the REST API

### For DevOps
Deploying and operating Rice Search:
1. [Deployment Guide](deployment.md) - Production deployment
2. [Configuration](configuration.md) - Environment variables and tuning
3. [Operations Guide](operations.md) - Monitoring and maintenance
4. [Security Guide](security.md) - Securing your installation

---

## 🔗 External Resources

- **GitHub**: [rice-search repository](https://github.com/yourusername/rice-search)
- **Issues**: [Report bugs or request features](https://github.com/yourusername/rice-search/issues)
- **License**: [CC BY-NC-SA 4.0](../LICENSE.md)

---

## 📝 Documentation Status

| Document | Status | Last Updated |
|----------|--------|--------------|
| Getting Started | ✅ Complete | 2026-01-06 |
| Architecture | ✅ Complete | 2026-01-06 |
| Configuration | ✅ Complete | 2026-01-06 |
| CLI Guide | ✅ Complete | 2026-01-06 |
| API Reference | ✅ Complete | 2026-01-06 |
| Development | ✅ Complete | 2026-01-06 |
| Deployment | ✅ Complete | 2026-01-06 |
| Testing | ✅ Complete | 2026-01-06 |
| Operations | ✅ Complete | 2026-01-06 |
| Troubleshooting | ✅ Complete | 2026-01-06 |
| Security | ✅ Complete | 2026-01-06 |
| Build & Release | ✅ Complete | 2026-01-06 |

---

## 💡 Tips

- **New to Rice Search?** Start with the [Getting Started](getting-started.md) guide
- **Need help?** Check [Troubleshooting](troubleshooting.md) for common issues
- **Want to contribute?** See the [Development Guide](development.md)
- **Looking for API details?** Visit the [API Reference](api.md)

---

<div align="center">

**[Back to Main README](../README.md)**

Made with ❤️ by the Rice Search Team

</div>
