# JIT Bot Helm Chart

A Helm chart for deploying JIT Bot - Just-In-Time Access Tool for AWS EKS clusters with Slack integration.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- Slack application configured with appropriate permissions

## Installation

### Add Helm Repository

```bash
helm repo add jit-bot https://rebelopsio.github.io/jit-bot/
helm repo update
```

### Install the Chart

```bash
helm install my-jit-bot jit-bot/jit-bot
```

### Install with Custom Values

```bash
helm install my-jit-bot jit-bot/jit-bot \
  --set secrets.slack.botToken="xoxb-your-bot-token" \
  --set secrets.slack.signingSecret="your-signing-secret" \
  --set env[0].name="AWS_REGION" \
  --set env[0].value="us-west-2"
```

## Configuration

The following table lists the configurable parameters and their default values.

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of replicas | `2` |
| `image.repository` | Image repository | `ghcr.io/rebelopsio/jit-bot/jit-server` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `image.tag` | Image tag (defaults to chart appVersion) | `""` |
| `service.type` | Kubernetes service type | `ClusterIP` |
| `service.port` | Service port | `80` |
| `service.targetPort` | Container port | `8080` |
| `resources.limits.cpu` | CPU limit | `500m` |
| `resources.limits.memory` | Memory limit | `256Mi` |
| `resources.requests.cpu` | CPU request | `100m` |
| `resources.requests.memory` | Memory request | `128Mi` |
| `secrets.slack.botToken` | Slack bot OAuth token | `""` |
| `secrets.slack.signingSecret` | Slack app signing secret | `""` |
| `env` | Additional environment variables | See values.yaml |
| `autoscaling.enabled` | Enable horizontal pod autoscaling | `false` |
| `ingress.enabled` | Enable ingress | `false` |
| `monitoring.enabled` | Enable monitoring | `true` |

## Required Configuration

Before deploying, you must configure the following:

1. **Slack Bot Token**: OAuth token for your Slack app
2. **Slack Signing Secret**: Signing secret for request verification

### Example values.yaml

```yaml
secrets:
  slack:
    botToken: "xoxb-1234567890123-1234567890123-abcdefghijklmnopqrstuvwx"
    signingSecret: "abcdef1234567890abcdef1234567890"

env:
  - name: PORT
    value: "8080"
  - name: AWS_REGION
    value: "us-east-1"

ingress:
  enabled: true
  className: "nginx"
  hosts:
    - host: jit.example.com
      paths:
        - path: /
          pathType: ImplementationSpecific
  tls:
    - secretName: jit-bot-tls
      hosts:
        - jit.example.com
```

## Uninstalling the Chart

```bash
helm uninstall my-jit-bot
```

## Architecture

JIT Bot consists of:

- **JIT Server**: Main application handling Slack integration and access requests
- **JIT Operator**: Kubernetes operator managing access policies and resources

## Security

The chart includes security best practices:

- Non-root user (65534)
- Read-only root filesystem
- Dropped capabilities
- Security contexts
- Pod anti-affinity rules

## Monitoring

When monitoring is enabled, the chart creates:

- ServiceMonitor for Prometheus scraping
- Metrics endpoint on `/metrics`

## Support

For issues and questions, please visit the [GitHub repository](https://github.com/rebelopsio/jit-bot).