# mirth-operator

A Kubernetes operator for monitoring and managing [Mirth Connect](https://www.nextgen.com/solutions/interoperability/mirth-integration-engine) (NextGen Connect) instances.

Mirth Connect is widely used in healthcare integration but is a black box from a Kubernetes perspective. K8s probes can only check if the JVM is alive -- they can't verify channels are running, queues are draining, or downstream systems are reachable. This operator bridges that gap.

## Features

- **Observability** -- Polls Mirth's REST API and exposes channel-level Prometheus metrics (message counts, queue depth, channel state)
- **CRD Status** -- Updates `MirthInstance` status visible via `kubectl get mirthinstances`, including per-channel health
- **Auto-Remediation** -- Automatically restarts stopped or errored channels with configurable cooldown, max attempts, and exclusion lists
- **Kubernetes Events** -- Emits K8s events for state transitions and remediation actions

## CRD: MirthInstance

```yaml
apiVersion: mirth.pyle.io/v1alpha1
kind: MirthInstance
metadata:
  name: mirth-dev
  namespace: integrations
spec:
  connection:
    host: mirth-connect.integrations.svc.cluster.local
    port: 8443
    tls:
      insecureSkipVerify: true
    authSecretRef:
      name: mirth-admin-credentials   # Secret with "username" and "password" keys
  monitoring:
    intervalSeconds: 30
    metrics:
      enabled: true
  remediation:
    enabled: true
    restartStoppedChannels: true
    restartErroredChannels: true
    maxRestartAttempts: 3
    cooldownSeconds: 300
    excludeChannels: []
```

### Status output

```
$ kubectl get mirthinstances
NAME        CONNECTED   CHANNELS   HEALTHY   AGE
mirth-dev   True        12         12        5d
```

## Prometheus Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `mirth_up` | Gauge | `instance` | API reachable (0/1) |
| `mirth_channel_status` | Gauge | `instance`, `channel`, `state` | 1 if channel is in this state |
| `mirth_channel_messages_received_total` | Gauge | `instance`, `channel` | Messages received |
| `mirth_channel_messages_sent_total` | Gauge | `instance`, `channel` | Messages sent |
| `mirth_channel_messages_errored_total` | Gauge | `instance`, `channel` | Messages errored |
| `mirth_channel_messages_queued` | Gauge | `instance`, `channel` | Current queue depth |
| `mirth_channel_messages_filtered_total` | Gauge | `instance`, `channel` | Messages filtered |
| `mirth_channels_total` | Gauge | `instance` | Total channels |
| `mirth_channels_healthy` | Gauge | `instance` | Channels in STARTED state |
| `mirth_remediation_total` | Counter | `instance`, `channel`, `result` | Remediation attempts |
| `mirth_jvm_heap_used_bytes` | Gauge | `instance` | JVM heap usage |

## Getting Started

### Prerequisites

- Go 1.23+
- Docker
- kubectl
- Access to a Kubernetes cluster

### Local Development

```bash
# Install CRDs
make install

# Run operator locally against your current kubeconfig
make run

# Apply sample CR
kubectl apply -f config/samples/mirth_v1alpha1_mirthinstance.yaml
```

### Deploy with Helm

```bash
helm install mirth-operator chart/mirth-operator \
  --namespace mirth-system \
  --create-namespace
```

### Deploy with Kustomize

```bash
make docker-build docker-push IMG=ghcr.io/pyle-cloud-technologies/mirth-operator:latest
make deploy IMG=ghcr.io/pyle-cloud-technologies/mirth-operator:latest
```

### Run Tests

```bash
# Unit tests (mirth client + remediation)
go test ./internal/mirth/... ./internal/remediation/... -v

# All tests (requires envtest binaries)
make test
```

## Architecture

```
MirthInstance CR
       |
       v
  Reconciler (polling loop)
       |
       +-- Mirth REST API Client --> Mirth Connect server
       |       GET /api/server/status
       |       GET /api/system/stats
       |       GET /api/channels/statuses
       |
       +-- Metrics Collector --> Prometheus /metrics endpoint
       |
       +-- Remediation Handler
       |       Evaluate: which channels need action?
       |       Execute: POST /api/channels/{id}/_start or _restart
       |
       +-- Status Update --> MirthInstance.status (kubectl visible)
       +-- K8s Events --> event stream
```

## Helm Chart Values

| Parameter | Default | Description |
|-----------|---------|-------------|
| `image.repository` | `ghcr.io/pyle-cloud-technologies/mirth-operator` | Container image |
| `image.tag` | `appVersion` | Image tag |
| `replicaCount` | `1` | Number of replicas |
| `leaderElection.enabled` | `true` | Enable leader election |
| `metrics.enabled` | `true` | Enable metrics endpoint |
| `metrics.port` | `8080` | Metrics port |
| `metrics.serviceMonitor.enabled` | `false` | Create ServiceMonitor |
| `resources.requests.cpu` | `10m` | CPU request |
| `resources.requests.memory` | `64Mi` | Memory request |
| `resources.limits.cpu` | `500m` | CPU limit |
| `resources.limits.memory` | `128Mi` | Memory limit |

## License

Copyright 2026 Pyle Cloud Technologies. Licensed under the Apache License 2.0.
