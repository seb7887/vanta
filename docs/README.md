# Vanta Documentation

Welcome to the comprehensive documentation for Vanta, the high-performance OpenAPI mock server. This documentation is organized to help you quickly find what you need, whether you're an end user, developer, or operator.

## 📖 Documentation Structure

### 🚀 User Guide
Documentation for end users who want to use Vanta for API mocking and testing.

| Document | Description | Quick Link |
|----------|-------------|------------|
| **[Getting Started](user-guide/getting-started.md)** | Quick setup and first steps with Vanta | ⭐ **Start here** |
| **[Configuration Reference](user-guide/configuration.md)** | Complete configuration options and examples | 🔧 Essential |
| **[OpenAPI & Data Generation](user-guide/openapi-and-data-generation.md)** | How Vanta generates realistic mock data | 📊 Core feature |
| **[Middleware & Metrics](user-guide/middleware-and-metrics.md)** | Middleware stack and performance monitoring | 📈 Monitoring |
| **[Chaos Engineering](user-guide/chaos-guide.md)** | Inject failures and latency for testing | 🔥 Testing |
| **[Recording & Replay](user-guide/recording-and-replay.md)** | Capture and replay HTTP traffic | 📹 Traffic tools |
| **[Validation Guide](user-guide/validation-guide.md)** | Request/response validation against OpenAPI | ✅ Quality |
| **[TUI Guide](user-guide/tui-guide.md)** | Interactive terminal interface | 💻 Interface |
| **[Hot Reload & State](user-guide/hot-reload-and-state.md)** | Dynamic reloading and state management | 🔄 Advanced |

### 👨‍💻 Developer Guide
Documentation for developers contributing to or extending Vanta.

| Document | Description | Quick Link |
|----------|-------------|------------|
| **[Getting Started](developer-guide/getting-started.md)** | 5-minute developer setup | ⚡ Quick start |
| **[Architecture Overview](developer-guide/architecture.md)** | System design and component relationships | 🏗️ Understanding |
| **[Development Setup](developer-guide/development-setup.md)** | Complete development environment setup | 🛠️ Deep dive |

### 📚 API Reference
Technical references for plugins, APIs, and extensibility.

| Document | Description | Quick Link |
|----------|-------------|------------|
| **[Plugins Guide](api-reference/plugins-guide.md)** | Plugin system and custom development | 🔌 Extensibility |
| **[Built-in Plugins](api-reference/builtin-plugins.md)** | Reference for included plugins | 📦 Ready-to-use |

### 🚢 Deployment
Operational documentation for production deployments.

| Document | Description | Quick Link |
|----------|-------------|------------|
| **[Docker & Kubernetes](deployment/docker-and-kubernetes.md)** | Container deployment and orchestration | 🐳 Containers |
| **[Production Best Practices](deployment/production-best-practices.md)** | Security, scaling, and operational guidance | 🏭 Production |
| **[Monitoring & Troubleshooting](deployment/monitoring-and-troubleshooting.md)** | Observability and problem-solving | 🔍 Operations |
| **[Performance Tuning](deployment/performance-tuning.md)** | Optimization and scaling strategies | ⚡ Performance |

## 🎯 Quick Navigation

### New to Vanta?
1. **[User Getting Started](user-guide/getting-started.md)** - Get Vanta running in 2 minutes
2. **[Configuration Reference](user-guide/configuration.md)** - Understand the configuration options
3. **[OpenAPI & Data Generation](user-guide/openapi-and-data-generation.md)** - Learn how realistic data is generated

### Want to Contribute?
1. **[Developer Getting Started](developer-guide/getting-started.md)** - 5-minute development setup
2. **[Architecture Overview](developer-guide/architecture.md)** - Understand the system design
3. **[Development Setup](developer-guide/development-setup.md)** - Complete development environment

### Deploying to Production?
1. **[Docker & Kubernetes](deployment/docker-and-kubernetes.md)** - Container deployment
2. **[Production Best Practices](deployment/production-best-practices.md)** - Security and scaling
3. **[Monitoring & Troubleshooting](deployment/monitoring-and-troubleshooting.md)** - Observability setup

### Need Advanced Features?
- **[Chaos Engineering](user-guide/chaos-guide.md)** - Failure injection testing
- **[Recording & Replay](user-guide/recording-and-replay.md)** - Traffic capture and replay
- **[Plugins Guide](api-reference/plugins-guide.md)** - Extend functionality
- **[Performance Tuning](deployment/performance-tuning.md)** - Optimize for high throughput

## 📋 Examples

All documentation references practical examples located in the `examples/` directory:

- `examples/petstore.yaml` - Standard OpenAPI specification for testing
- `examples/quickstart/` - Minimal setup for getting started
- `examples/k8s/` - Kubernetes deployment manifests
- `examples/*-config.yaml` - Various configuration examples

## 🆘 Getting Help

### Documentation Issues
If you find errors or want to improve this documentation:
1. Check the [latest documentation](https://github.com/your-org/vanta/tree/main/docs)
2. Open an issue with the `documentation` label
3. Submit a pull request with improvements

### Usage Questions
- **Check existing documentation** - Most questions are answered here
- **Review examples** - Practical configurations in `examples/`
- **Use the TUI** - Interactive debugging with `vanta tui`
- **Enable debug logging** - Add `logging.level: debug` to your config

### Bug Reports and Feature Requests
Visit the [GitHub Issues](https://github.com/your-org/vanta/issues) page.

## 📄 Document Conventions

### Symbols Used
- ⭐ **Essential** - Must-read for all users
- 🔧 **Configuration** - Configuration-focused content
- 📊 **Features** - Core functionality explanations
- 🛠️ **Development** - Developer-oriented content
- 🚀 **Deployment** - Production and operational content

### Code Examples
All code examples are tested and use the configurations provided in the `examples/` directory. You can copy and run them directly.

### Version Information
This documentation is maintained for the latest version of Vanta. Version-specific information is noted where applicable.

---

**Need help?** Start with the [Getting Started guide](user-guide/getting-started.md) or check the [Architecture Overview](developer-guide/architecture.md) to understand how Vanta works.