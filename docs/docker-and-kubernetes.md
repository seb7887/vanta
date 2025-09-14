# Docker & Kubernetes

## Docker
Build the image and run the server:
```
docker build -t vanta .
docker run --rm -p 8080:8080 \
  -v $PWD/examples:/app/examples \
  vanta start --config /app/examples/docker-config.yaml
```

Notes:
- The image uses a non‑root user and includes example configs.
- Healthcheck runs `start --config /app/examples/docker-config.yaml --health-check`.

## Kubernetes
Use the manifests in `examples/k8s/`.
```
kubectl apply -f examples/k8s/
kubectl get pods -l app=vanta
kubectl port-forward service/vanta 8080:80
curl http://localhost:8080/health
```

Features:
- ClusterIP/LoadBalancer/NodePort services
- Ingress (HTTP/HTTPS) with TLS option
- Liveness/Readiness probes
- Prometheus annotations
- Scaling examples

See `examples/k8s/README.md` for detailed steps, troubleshooting, and cleanup.

