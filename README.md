# OpenClaw Operator

Kubernetes Operator for deploying and managing OpenClaw AI agent instances on ToC platform.

## Features

- ✅ Declarative OpenClaw instance management via CRD
- ✅ Automatic health check and self-healing
- ✅ hostPort: 18789 exposure (simple and direct)
- ✅ Single namespace deployment
- ✅ Status sync to backend database
- ✅ Config hot-reload with automatic restart

## Quick Start

```bash
# Install CRDs
make install

# Deploy operator
make deploy

# Create an instance
kubectl apply -f config/samples/openclaw_v1alpha1_openclawinstance.yaml
```

## Architecture

```
┌─────────────────┐     Watch      ┌──────────────────┐
│  K8s API Server │ ◄───────────── │  OpenClaw        │
│  - CRD          │                │  Operator        │
│  - Pod          │ ─────────────► │  (Deployment)    │
└─────────────────┘   Events       └──────────────────┘
                                                 │
                                                 │ Patch Status
                                                 ▼
                                    ┌──────────────────┐
                                    │  OpenClawInstance│
                                    │  CR (Status)     │
                                    └──────────────────┘
```

## Development

```bash
# Run locally
make run

# Build image
make docker-build

# Deploy to cluster
make deploy
```

## License

Apache 2.0
