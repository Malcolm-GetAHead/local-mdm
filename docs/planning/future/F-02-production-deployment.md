# F-02: Production Deployment & High Availability

**Priority**: High  
**Effort**: 2-3 days  
**Score Impact**: +0.20 points  
**Status**: Future (Kubernetes marked as future in scope)

---

## Gap Analysis

### Current State
- Docker support (S1-02)
- Docker Compose for development
- Health check endpoints (S5-06)
- Basic deployment documentation (S5-04)

### Missing
- Kubernetes manifests
- Helm charts for easy deployment
- High availability configuration
- Load balancer setup
- Zero-downtime deployment strategy
- Auto-scaling configuration
- Production troubleshooting guide

### Impact
Without production deployment tooling:
- Manual deployment is error-prone
- No horizontal scaling capability
- Downtime during updates
- No automatic failover
- Difficult to manage multiple environments

---

## Proposed Solution

### 1. Kubernetes Manifests

**Deployments**:
```yaml
# k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: localmdm
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  template:
    spec:
      containers:
      - name: localmdm
        image: localmdm:latest
        ports:
        - containerPort: 8080
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 5
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
```

**Services**:
```yaml
# k8s/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: localmdm
spec:
  type: LoadBalancer
  ports:
  - port: 443
    targetPort: 8080
  selector:
    app: localmdm
```

**Ingress**:
```yaml
# k8s/ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: localmdm
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  tls:
  - hosts:
    - mdm.example.com
    secretName: localmdm-tls
  rules:
  - host: mdm.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: localmdm
            port:
              number: 8080
```

**ConfigMap**:
```yaml
# k8s/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: localmdm-config
data:
  config.yaml: |
    server:
      port: 8080
    database:
      host: postgres.default.svc.cluster.local
      port: 5432
      name: localmdm
```

**Secrets**:
```yaml
# k8s/secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: localmdm-secrets
type: Opaque
data:
  db_password: <base64-encoded>
  jwt_secret: <base64-encoded>
```

### 2. Helm Chart

**Chart Structure**:
```
helm/localmdm/
├── Chart.yaml
├── values.yaml
├── values-production.yaml
├── values-staging.yaml
├── templates/
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── ingress.yaml
│   ├── configmap.yaml
│   ├── secret.yaml
│   ├── hpa.yaml
│   ├── pdb.yaml
│   └── serviceaccount.yaml
```

**values.yaml**:
```yaml
replicaCount: 3

image:
  repository: localmdm
  tag: latest
  pullPolicy: IfNotPresent

service:
  type: LoadBalancer
  port: 443

ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: mdm.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: localmdm-tls
      hosts:
        - mdm.example.com

resources:
  requests:
    memory: 256Mi
    cpu: 250m
  limits:
    memory: 512Mi
    cpu: 500m

autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70
  targetMemoryUtilizationPercentage: 80

database:
  host: postgres.default.svc.cluster.local
  port: 5432
  name: localmdm
  # Password from secret

keycloak:
  url: https://keycloak.example.com
  realm: localmdm
  clientId: localmdm-api
  # Client secret from secret
```

### 3. High Availability Configuration

**Components**:
- 3+ application replicas
- Load balancer (AWS ALB, GCP Load Balancer, NGINX Ingress)
- PostgreSQL with read replicas (managed service: RDS, Cloud SQL)
- Redis for session/cache (optional, ElastiCache, Memorystore)
- Horizontal Pod Autoscaler (HPA)
- Pod Disruption Budget (PDB)

**PDB Example**:
```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: localmdm-pdb
spec:
  minAvailable: 2
  selector:
    matchLabels:
      app: localmdm
```

**HPA Example**:
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: localmdm-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: localmdm
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

### 4. Zero-Downtime Deployment

**Strategy**: Rolling Update
- Deploy new version gradually
- Health checks ensure new pods are ready
- Old pods terminated only after new pods healthy
- Rollback capability if health checks fail

**Blue-Green Deployment** (alternative):
- Deploy new version alongside old version
- Switch traffic to new version
- Keep old version for quick rollback
- Remove old version after validation

**Canary Deployment** (advanced):
- Deploy new version to small percentage of traffic (10%)
- Monitor metrics and errors
- Gradually increase traffic to new version
- Rollback if issues detected

---

## Implementation Tasks

### Task 1: Kubernetes Manifests (1 day)
- Create deployment, service, ingress, configmap, secret manifests
- Configure health checks and resource limits
- Set up HPA and PDB
- Test on local Kubernetes (minikube, kind)

### Task 2: Helm Chart (1 day)
- Create Helm chart structure
- Parameterize all configuration
- Create values files for dev, staging, production
- Test Helm install/upgrade/rollback
- Package and publish chart

### Task 3: Production Setup Guide (0.5 days)
- Document cloud provider setup (AWS EKS, GCP GKE, Azure AKS)
- Load balancer configuration
- TLS certificate setup (cert-manager)
- Database connection (RDS, Cloud SQL)
- Secrets management (AWS Secrets Manager, GCP Secret Manager)
- Monitoring integration (Prometheus, Grafana)

### Task 4: Troubleshooting Guide (0.5 days)
- Common deployment issues
- Pod crash loop debugging
- Database connection issues
- Certificate problems
- Performance tuning
- Scaling issues

---

## Security Hardening (from SECURITY.md TODOs)

The following items were identified as TODOs in `docs/SECURITY.md` during Sprint 4 retrospective and belong in this production deployment scope:

- **Dependency scanning**: Set up automated vulnerability scanning (Dependabot, Snyk, or `govulncheck`) in CI pipeline
- **Code signing**: Sign release binaries and container images for supply chain integrity
- **Network segmentation**: Document production network architecture (VPC, security groups, private subnets for DB/services)
- **Security contact email**: Set up `security@localmdm.dev` for responsible disclosure

---

## Acceptance Criteria

- [ ] Kubernetes manifests deploy successfully
- [ ] Helm chart installs with custom values
- [ ] 3 replicas running with load balancing
- [ ] Health checks pass (liveness and readiness)
- [ ] Zero-downtime rolling update works
- [ ] HPA scales up under load
- [ ] PDB prevents all pods from being evicted
- [ ] TLS termination at ingress works
- [ ] Secrets loaded from external secret manager
- [ ] Prometheus metrics scraped successfully

---

## Cloud Provider Examples

### AWS EKS
```bash
# Create EKS cluster
eksctl create cluster --name localmdm --region us-east-1 --nodes 3

# Install cert-manager
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Install NGINX Ingress
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/aws/deploy.yaml

# Deploy Local MDM
helm install localmdm ./helm/localmdm -f values-production.yaml
```

### GCP GKE
```bash
# Create GKE cluster
gcloud container clusters create localmdm --num-nodes=3 --region=us-central1

# Install cert-manager
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Deploy Local MDM
helm install localmdm ./helm/localmdm -f values-production.yaml
```

### Azure AKS
```bash
# Create AKS cluster
az aks create --resource-group localmdm --name localmdm --node-count 3

# Get credentials
az aks get-credentials --resource-group localmdm --name localmdm

# Deploy Local MDM
helm install localmdm ./helm/localmdm -f values-production.yaml
```

---

## Monitoring & Alerting

**Prometheus ServiceMonitor**:
```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: localmdm
spec:
  selector:
    matchLabels:
      app: localmdm
  endpoints:
  - port: http
    path: /metrics
    interval: 30s
```

**Grafana Dashboard**:
- Import dashboard from S5-06 metrics
- Add Kubernetes-specific metrics (pod restarts, CPU/memory usage)

---

## Cost Optimization

**Development**:
- 1 replica, smaller instance types
- Spot instances (AWS, GCP)
- Auto-shutdown during off-hours

**Staging**:
- 2 replicas, medium instance types
- Shared database with development

**Production**:
- 3+ replicas, production-grade instances
- Dedicated database with read replicas
- Multi-AZ deployment

---

## Security Considerations

- Network policies to restrict pod-to-pod communication
- Pod security policies/standards
- RBAC for Kubernetes API access
- Secrets encryption at rest
- Image scanning (Trivy, Snyk)
- Runtime security (Falco)

---

## Future Enhancements

- Multi-region deployment
- GitOps with ArgoCD or Flux
- Service mesh (Istio, Linkerd)
- Chaos engineering (Chaos Mesh)
- Cost monitoring and optimization
- Automated backup and restore

---

## References

- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [Helm Documentation](https://helm.sh/docs/)
- [S5-04: Deployment Guide](../sprint-5-ui-and-polish/S5-04-deployment.md)
- [S5-06: Observability](../sprint-5-ui-and-polish/S5-06-observability.md)
