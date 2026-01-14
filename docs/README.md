# Octopus Documentation

Welcome to the Octopus documentation. This guide will help you install, configure, and use Octopus for VM migration and disaster recovery.

## Quick Links

| Document | Description |
|----------|-------------|
| [Installation Guide](installation.md) | How to install Octopus |
| [User Guide](user-guide.md) | How to use Octopus |
| [Size Estimation Guide](size-estimation.md) | VXRail/vSAN size estimation methodology |
| [API Reference](api-reference.md) | REST API documentation |
| [Architecture](architecture.md) | Technical architecture |

## Getting Started

1. **Install Octopus** - Follow the [Installation Guide](installation.md)
2. **Configure** - Set up authentication and providers
3. **Add Sources** - Connect to VMware vCenter
4. **Create Migrations** - Start migrating VMs

## Quick Install

### Linux/macOS
```bash
git clone https://github.com/sp00nznet/octopus.git
cd octopus
./scripts/install.sh
```

### Windows
```powershell
git clone https://github.com/sp00nznet/octopus.git
cd octopus
.\scripts\install.ps1
```

### Docker
```bash
git clone https://github.com/sp00nznet/octopus.git
cd octopus
make docker-run
```

## Support

- **Issues**: [GitHub Issues](https://github.com/sp00nznet/octopus/issues)
- **Discussions**: [GitHub Discussions](https://github.com/sp00nznet/octopus/discussions)

## Contributing

We welcome contributions! Please see the [Contributing Guide](../CONTRIBUTING.md) for details.
