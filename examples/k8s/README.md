# Kubernetes Deployment for Vanta

This directory contains Kubernetes manifests for deploying Vanta OpenAPI Mocker to a Kubernetes cluster.

## Files Overview

- `configmap.yaml` - Configuration and OpenAPI specification
- `deployment.yaml` - Main deployment with PodDisruptionBudget
- `service.yaml` - Services (ClusterIP, LoadBalancer, NodePort)
- `ingress.yaml` - Ingress configurations (HTTP and HTTPS)

## Quick Start

### 1. Deploy to Kubernetes

```bash
# Apply all manifests
kubectl apply -f examples/k8s/

# Or apply individually in order
kubectl apply -f examples/k8s/configmap.yaml
kubectl apply -f examples/k8s/deployment.yaml
kubectl apply -f examples/k8s/service.yaml
kubectl apply -f examples/k8s/ingress.yaml
```

### 2. Verify Deployment

```bash
# Check deployment status
kubectl get deployments vanta
kubectl get pods -l app=vanta

# Check services
kubectl get services -l app=vanta

# Check ingress
kubectl get ingress vanta
```

### 3. Test the API

```bash
# Port forward for local testing
kubectl port-forward service/vanta 8080:80

# Test endpoints
curl http://localhost:8080/health
curl http://localhost:8080/pets
```

## Configuration Options

### Environment Variables

The deployment supports these environment variables:

- `PORT` - Server port (default: 8080)
- `LOG_LEVEL` - Logging level (default: info)

### Resource Limits

Default resource configuration:

```yaml
resources:
  limits:
    cpu: 500m
    memory: 256Mi
  requests:
    cpu: 100m
    memory: 128Mi
```

Adjust based on your workload requirements.

### Health Checks

The deployment includes comprehensive health checks:

- **Liveness Probe**: Ensures container is running
- **Readiness Probe**: Ensures container is ready to serve traffic
- **Health Endpoint**: `/health` for external monitoring

## Service Types

### ClusterIP (Default)
- Internal cluster access only
- Best for microservice communication

### LoadBalancer
- External access via cloud load balancer
- Automatically provisions external IP

### NodePort
- Access via node IP and specific port (30080)
- Good for development and testing

## Ingress Configuration

### Basic HTTP Ingress
- Host: `vanta.example.com` (update to your domain)
- Supports rate limiting and CORS

### HTTPS Ingress with TLS
- Automatic SSL certificate management with cert-manager
- Security headers included
- Force HTTPS redirect

### Ingress Features
- Rate limiting (10 RPS)
- CORS configuration
- Custom headers
- Health check routing
- Timeout settings

## Customization

### Update OpenAPI Specification

Edit the `petstore.yaml` section in `configmap.yaml` or mount your own spec:

```yaml
# Add custom spec volume
volumes:
- name: custom-spec
  configMap:
    name: my-custom-spec
```

### Configuration Changes

Modify the `config.yaml` section in `configmap.yaml` to adjust:

- Server settings
- Enable/disable features (chaos, recording, plugins)
- Logging configuration
- CORS settings

### Scaling

```bash
# Scale to 3 replicas
kubectl scale deployment vanta --replicas=3

# Auto-scaling (requires metrics server)
kubectl autoscale deployment vanta --cpu-percent=70 --min=2 --max=10
```

## Monitoring

### Prometheus Integration

The deployment includes Prometheus annotations:

```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "8080"
  prometheus.io/path: "/metrics"
```

### Metrics Endpoint

Access metrics at `/metrics` endpoint for monitoring.

## Security

### Security Features
- Non-root user (UID 1001)
- Read-only root filesystem
- Dropped capabilities
- Security context policies
- PodDisruptionBudget for availability

### Network Policies (Optional)

Create network policies to restrict traffic:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: vanta-network-policy
spec:
  podSelector:
    matchLabels:
      app: vanta
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector: {}
    ports:
    - protocol: TCP
      port: 8080
```

## Troubleshooting

### Common Issues

1. **Pod not starting**
   ```bash
   kubectl describe pod <pod-name>
   kubectl logs <pod-name>
   ```

2. **Service not accessible**
   ```bash
   kubectl get endpoints vanta
   kubectl describe service vanta
   ```

3. **Ingress not working**
   ```bash
   kubectl describe ingress vanta
   kubectl get ingress vanta
   ```

### Logs

```bash
# View application logs
kubectl logs -l app=vanta -f

# View logs for specific pod
kubectl logs <pod-name> -f
```

## Cleanup

```bash
# Remove all resources
kubectl delete -f examples/k8s/

# Or remove individually
kubectl delete ingress vanta vanta-tls
kubectl delete service vanta vanta-external vanta-nodeport
kubectl delete deployment vanta
kubectl delete configmap vanta-config vanta-spec
kubectl delete poddisruptionbudget vanta-pdb
```