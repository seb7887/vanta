# Production Deployment Best Practices

This guide covers essential considerations for deploying Vanta in production environments, including security, performance, reliability, and operational excellence.

## 🔒 Security Best Practices

### Container Security

#### Use Non-Root User
```dockerfile
# Already implemented in Vanta's Dockerfile
USER nonroot:nonroot
```

#### Minimal Base Image
```dockerfile
# Use minimal base images
FROM gcr.io/distroless/static:nonroot
# or
FROM alpine:latest
```

#### Image Scanning
```bash
# Scan images for vulnerabilities
docker scan vanta:latest

# Or use tools like Trivy
trivy image vanta:latest
```

### Network Security

#### TLS/HTTPS Configuration
```yaml
# config/production.yaml
server:
  tls:
    enabled: true
    cert_file: "/etc/ssl/certs/vanta.crt"
    key_file: "/etc/ssl/private/vanta.key"
    min_version: "1.2"
    max_version: "1.3"
```

#### CORS Configuration
```yaml
middleware:
  cors:
    enabled: true
    allow_origins: ["https://yourapp.com", "https://api.yourapp.com"]
    allow_methods: ["GET", "POST", "PUT", "DELETE"]
    allow_headers: ["Content-Type", "Authorization", "X-Request-ID"]
    expose_headers: ["X-Request-ID"]
    allow_credentials: false
    max_age: 86400
```

#### Rate Limiting
```yaml
plugins:
  - name: rate_limit
    enabled: true
    config:
      ip_requests_per_second: 100.0
      ip_burst: 200
      key_requests_per_second: 1000.0
      key_burst: 2000
      cleanup_interval: "1m"
```

### Authentication and Authorization

#### JWT Authentication
```yaml
plugins:
  - name: auth
    enabled: true
    config:
      jwt_secret: "${JWT_SECRET}"  # Use environment variable
      auth_header: "Authorization"
      public_endpoints:
        - "/__health"
        - "/__info"
        - "/metrics"
      validate_claims: true
      required_claims: ["sub", "exp"]
```

#### API Key Authentication
```yaml
plugins:
  - name: auth
    enabled: true
    config:
      api_key_header: "X-API-Key"
      api_keys:
        - "${API_KEY_1}"
        - "${API_KEY_2}"
      public_endpoints: ["/__health"]
```

### Secrets Management

#### Environment Variables
```bash
# Use environment variables for sensitive data
export JWT_SECRET="your-super-secret-jwt-key"
export API_KEYS="key1,key2,key3"
export DATABASE_URL="postgresql://user:pass@host:port/db"
```

#### Kubernetes Secrets
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: vanta-secrets
type: Opaque
data:
  jwt-secret: <base64-encoded-secret>
  api-keys: <base64-encoded-keys>
```

## ⚡ Performance Optimization

### Server Configuration

#### Connection Limits
```yaml
server:
  port: 8080
  host: "0.0.0.0"
  read_timeout: "30s"
  write_timeout: "30s"
  idle_timeout: "120s"
  max_conns_per_ip: 1000
  concurrency: 10000
  max_request_body_size: "10MB"
```

#### Resource Limits
```yaml
# Kubernetes resource limits
resources:
  limits:
    cpu: "2"
    memory: "4Gi"
  requests:
    cpu: "500m"
    memory: "1Gi"
```

### Data Generation Optimization

#### Efficient Configuration
```yaml
mock:
  seed: 12345  # Deterministic generation
  prefer_examples: true  # Use spec examples when available
  max_depth: 5  # Limit nested object depth
  default_array_size: 10  # Reasonable array sizes
  max_array_size: 100
  locale: "en_US"
  cache_generated_data: true  # Cache frequently generated data
```

### Caching Strategy

#### Response Caching
```yaml
# Enable caching for static responses
plugins:
  - name: cache
    enabled: true
    config:
      ttl: "5m"
      max_size: "100MB"
      cache_headers: true
```

#### Memory Management
```yaml
# Optimize memory usage
server:
  gc_percent: 100  # Go garbage collection target
  max_memory: "2GB"
```

## 🏗️ Infrastructure Setup

### Load Balancing

#### NGINX Configuration
```nginx
upstream vanta_backend {
    server vanta-1:8080 max_fails=3 fail_timeout=30s;
    server vanta-2:8080 max_fails=3 fail_timeout=30s;
    server vanta-3:8080 max_fails=3 fail_timeout=30s;
}

server {
    listen 80;
    server_name api-mock.yourcompany.com;

    location / {
        proxy_pass http://vanta_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Timeouts
        proxy_connect_timeout 30s;
        proxy_send_timeout 30s;
        proxy_read_timeout 30s;

        # Health checks
        proxy_next_upstream error timeout http_500 http_502 http_503;
    }

    location /__health {
        proxy_pass http://vanta_backend;
        access_log off;
    }
}
```

#### HAProxy Configuration
```
backend vanta_servers
    balance roundrobin
    option httpchk GET /__health
    server vanta1 vanta-1:8080 check
    server vanta2 vanta-2:8080 check
    server vanta3 vanta-3:8080 check

frontend vanta_frontend
    bind *:80
    default_backend vanta_servers
```

### Kubernetes Deployment

#### Production Deployment
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: vanta
  labels:
    app: vanta
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  selector:
    matchLabels:
      app: vanta
  template:
    metadata:
      labels:
        app: vanta
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 65532
        fsGroup: 65532
      containers:
      - name: vanta
        image: vanta:latest
        ports:
        - containerPort: 8080
          name: http
        - containerPort: 9090
          name: metrics
        env:
        - name: JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: vanta-secrets
              key: jwt-secret
        resources:
          limits:
            cpu: "2"
            memory: "4Gi"
          requests:
            cpu: "500m"
            memory: "1Gi"
        livenessProbe:
          httpGet:
            path: /__health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 3
        readinessProbe:
          httpGet:
            path: /__health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
          timeoutSeconds: 3
          failureThreshold: 2
        volumeMounts:
        - name: config
          mountPath: /app/config
          readOnly: true
        - name: specs
          mountPath: /app/specs
          readOnly: true
      volumes:
      - name: config
        configMap:
          name: vanta-config
      - name: specs
        configMap:
          name: vanta-specs
```

#### Horizontal Pod Autoscaler
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: vanta-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: vanta
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

## 📊 Monitoring and Observability

### Metrics Configuration

#### Prometheus Metrics
```yaml
metrics:
  enabled: true
  port: 9090
  path: "/metrics"
  prometheus:
    enabled: true
    namespace: "vanta"
    subsystem: "mock_server"
    buckets: [0.001, 0.01, 0.1, 0.5, 1.0, 2.5, 5.0, 10.0]
```

#### Custom Metrics
```yaml
metrics:
  custom_metrics:
    - name: "requests_by_endpoint"
      type: "counter"
      labels: ["method", "endpoint", "status"]
    - name: "response_time_by_endpoint"
      type: "histogram"
      labels: ["method", "endpoint"]
```

### Logging Configuration

#### Structured Logging
```yaml
logging:
  level: "info"  # info, debug, warn, error
  format: "json"
  output: "stdout"
  development: false
  sampling:
    initial: 100
    thereafter: 100
  stack_trace_level: "error"
```

#### Log Aggregation
```yaml
# Fluentd/Fluent Bit configuration for log shipping
logging:
  fields:
    service: "vanta"
    environment: "production"
    version: "v1.0.0"
```

### Health Checks

#### Comprehensive Health Check
```yaml
# Add to your configuration
health:
  enabled: true
  checks:
    - name: "openapi_spec"
      type: "file_exists"
      path: "/app/specs/api.yaml"
    - name: "configuration"
      type: "config_valid"
    - name: "memory_usage"
      type: "memory_threshold"
      threshold: "80%"
    - name: "disk_space"
      type: "disk_threshold"
      path: "/tmp"
      threshold: "90%"
```

## 🔄 Deployment Strategies

### Blue-Green Deployment

#### Deployment Script
```bash
#!/bin/bash
# blue-green-deploy.sh

NEW_VERSION=$1
CURRENT_SLOT=$(kubectl get service vanta -o jsonpath='{.spec.selector.slot}')
NEW_SLOT=$([ "$CURRENT_SLOT" = "blue" ] && echo "green" || echo "blue")

echo "Deploying version $NEW_VERSION to $NEW_SLOT slot"

# Deploy to inactive slot
kubectl set image deployment/vanta-$NEW_SLOT vanta=vanta:$NEW_VERSION

# Wait for rollout
kubectl rollout status deployment/vanta-$NEW_SLOT

# Health check
if kubectl exec deployment/vanta-$NEW_SLOT -- curl -f http://localhost:8080/__health; then
    # Switch traffic
    kubectl patch service vanta -p '{"spec":{"selector":{"slot":"'$NEW_SLOT'"}}}'
    echo "Successfully switched to $NEW_SLOT"
else
    echo "Health check failed, rolling back"
    exit 1
fi
```

### Canary Deployment

#### Istio Configuration
```yaml
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: vanta-canary
spec:
  http:
  - match:
    - headers:
        canary:
          exact: "true"
    route:
    - destination:
        host: vanta
        subset: v2
  - route:
    - destination:
        host: vanta
        subset: v1
      weight: 90
    - destination:
        host: vanta
        subset: v2
      weight: 10
```

## 🚨 Disaster Recovery

### Backup Strategy

#### Configuration Backup
```bash
#!/bin/bash
# backup-config.sh

DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_DIR="/backups/vanta/$DATE"

mkdir -p $BACKUP_DIR

# Backup configurations
kubectl get configmap vanta-config -o yaml > $BACKUP_DIR/config.yaml
kubectl get configmap vanta-specs -o yaml > $BACKUP_DIR/specs.yaml
kubectl get secret vanta-secrets -o yaml > $BACKUP_DIR/secrets.yaml

# Backup state (if using persistent state)
kubectl exec deployment/vanta -- tar czf - /app/state | gzip > $BACKUP_DIR/state.tar.gz

echo "Backup completed: $BACKUP_DIR"
```

#### Automated Backups
```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: vanta-backup
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: backup
            image: vanta-backup:latest
            command: ["/scripts/backup-config.sh"]
          restartPolicy: OnFailure
```

### Recovery Procedures

#### Configuration Recovery
```bash
#!/bin/bash
# restore-config.sh

BACKUP_DATE=$1
BACKUP_DIR="/backups/vanta/$BACKUP_DATE"

if [ ! -d "$BACKUP_DIR" ]; then
    echo "Backup directory not found: $BACKUP_DIR"
    exit 1
fi

# Restore configurations
kubectl apply -f $BACKUP_DIR/config.yaml
kubectl apply -f $BACKUP_DIR/specs.yaml
kubectl apply -f $BACKUP_DIR/secrets.yaml

# Restart deployment to pick up changes
kubectl rollout restart deployment/vanta

echo "Recovery completed from: $BACKUP_DIR"
```

## 📋 Pre-Production Checklist

### Security Checklist
- [ ] TLS/HTTPS enabled with valid certificates
- [ ] CORS properly configured for your domains
- [ ] Rate limiting enabled and tuned
- [ ] Authentication/authorization configured
- [ ] Secrets managed securely (not in config files)
- [ ] Container runs as non-root user
- [ ] Image vulnerability scanning completed
- [ ] Network policies implemented (if using Kubernetes)

### Performance Checklist
- [ ] Resource limits configured
- [ ] Connection limits tuned for expected load
- [ ] Timeouts configured appropriately
- [ ] Load balancer health checks working
- [ ] Horizontal scaling configured
- [ ] Metrics and monitoring enabled
- [ ] Performance testing completed

### Reliability Checklist
- [ ] Health checks configured and tested
- [ ] Graceful shutdown working
- [ ] Backup and recovery procedures tested
- [ ] Deployment strategy defined (blue-green/canary)
- [ ] Rollback procedures documented
- [ ] Alerting configured for critical metrics
- [ ] Log aggregation and retention configured

### Operational Checklist
- [ ] Documentation updated for production setup
- [ ] Runbooks created for common operations
- [ ] Team trained on monitoring and troubleshooting
- [ ] Emergency contacts and escalation procedures defined
- [ ] Change management process established
- [ ] Capacity planning completed

## 📈 Scaling Considerations

### Vertical Scaling
```yaml
# Increase resource limits
resources:
  limits:
    cpu: "4"      # Increased from 2
    memory: "8Gi" # Increased from 4Gi
  requests:
    cpu: "1"      # Increased from 500m
    memory: "2Gi" # Increased from 1Gi
```

### Horizontal Scaling
```yaml
# Increase replica count
spec:
  replicas: 10  # Increased from 3

# Adjust HPA settings
spec:
  minReplicas: 5   # Increased from 3
  maxReplicas: 20  # Increased from 10
```

### Performance Tuning
```yaml
server:
  concurrency: 50000      # Increased from 10000
  max_conns_per_ip: 5000  # Increased from 1000

mock:
  cache_generated_data: true
  max_cache_size: "500MB"
```

This comprehensive guide provides the foundation for running Vanta reliably and securely in production. Always test configurations in a staging environment before applying to production.