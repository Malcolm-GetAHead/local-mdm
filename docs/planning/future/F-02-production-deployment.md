# F-02: Production Deployment & High Availability

**Priority**: High  
**Effort**: 2-3 days  
**Score Impact**: +0.20 points  
**Status**: Future  
**Last Updated**: 2026-04-21

---

## Gap Analysis

### Current State
- Docker support (S1-02)
- Docker Compose for development
- Health check endpoints (S5-06)
- Basic deployment documentation (S5-04)

### Missing
- Production container orchestration (ECS Fargate)
- Managed database with read replicas (RDS)
- Secrets management integration (SSM Parameter Store)
- TLS termination (ACM + ALB)
- Auto-scaling configuration
- Zero-downtime deployment strategy
- CloudWatch logging and metrics forwarding
- Production troubleshooting guide

---

## Proposed Solution: AWS ECS Fargate

Local MDM is a single Go binary with a PostgreSQL dependency. ECS Fargate is the right fit — no cluster management overhead, pay-per-task pricing, and native integration with ALB, RDS, SSM, ACM, and CloudWatch.

### Architecture Overview

```
                    ┌─────────────────────┐
                    │   Route 53 (DNS)    │
                    └──────────┬──────────┘
                               │
                    ┌──────────▼──────────┐
                    │   ACM Certificate   │
                    │   (auto-renewing)   │
                    └──────────┬──────────┘
                               │
                    ┌──────────▼──────────┐
                    │   ALB (public)      │
                    │   TLS termination   │
                    └──────────┬──────────┘
                               │
              ┌────────────────┼────────────────┐
              │                │                │
     ┌────────▼───────┐ ┌─────▼────────┐ ┌─────▼────────┐
     │  ECS Task 1    │ │  ECS Task 2  │ │  ECS Task 3  │
     │  localmdm:8080 │ │  localmdm    │ │  localmdm    │
     │  cw-agent:9090 │ │  cw-agent    │ │  cw-agent    │
     └──┬──────┬──────┘ └──┬──────┬────┘ └──┬──────┬────┘
        │      │           │      │          │      │
        │      └───────────┼──────┴──────────┼──────┘
        │                  │          ┌──────┘
        │    ┌─────────────┼──────────┤
        │    │             │          │
        │  ┌─▼────────────┐  ┌───────▼────────┐
        │  │ NanoMDM (ECS) │  │ Keycloak (ECS) │
        │  │ Apple MDM     │  │ OIDC IdP       │
        │  │ protocol +    │  │ admin login,   │
        │  │ APNs push     │  │ JWT, RBAC      │
        │  └───────┬───────┘  └───────┬────────┘
        │          │                  │
        └──────────┼──────────────────┘
                   │
     ┌─────────────┼─────────────┐
     │                           │
     ▼                           ▼
┌────────────────┐    ┌──────────────────┐
│  RDS Primary   │────▶  RDS Read Replica │
│  (Writer pool) │    │  (Reader pool)    │
└────────────────┘    └──────────────────┘
```

**Services**:
- **localmdm** (ECS Fargate): the Go application — API server, policy engine, enrollment handlers. Each task runs a CloudWatch Agent sidecar for Prometheus metrics forwarding.
- **nanomdm** (ECS Fargate): Apple MDM protocol handler — receives device check-ins, delivers commands via APNs, calls back to Local MDM webhooks (`/checkin`, `/mdm`). Shares the same RDS database (NanoMDM's PostgreSQL schema coexists with Local MDM's tables). Configured with `NANOMDM_API_KEY` for authenticated command submission from Local MDM.
- **keycloak** (ECS Fargate): OIDC identity provider — admin/operator login, JWT issuance, role management (super_admin, admin, operator, viewer). Uses the same RDS instance (separate `keycloak` database). Exposed via ALB on a dedicated host or path (e.g., `auth.mdm.example.com`).
- **RDS PostgreSQL**: primary for writes, read replica for Local MDM's Reader pool. NanoMDM and Keycloak use the primary only (separate databases on the same instance).

---

### 1. ECS Task Definition

```json
{
  "family": "localmdm",
  "networkMode": "awsvpc",
  "requiresCompatibilities": ["FARGATE"],
  "cpu": "512",
  "memory": "1024",
  "executionRoleArn": "arn:aws:iam::ACCOUNT:role/localmdm-execution",
  "taskRoleArn": "arn:aws:iam::ACCOUNT:role/localmdm-task",
  "containerDefinitions": [
    {
      "name": "localmdm",
      "image": "ACCOUNT.dkr.ecr.REGION.amazonaws.com/localmdm:latest",
      "essential": true,
      "portMappings": [
        {"containerPort": 8080, "protocol": "tcp"},
        {"containerPort": 9090, "protocol": "tcp"}
      ],
      "healthCheck": {
        "command": ["CMD-SHELL", "curl -f http://localhost:8080/health || exit 1"],
        "interval": 30,
        "timeout": 5,
        "retries": 3,
        "startPeriod": 60
      },
      "secrets": [
        {"name": "DB_PASSWORD", "valueFrom": "arn:aws:ssm:REGION:ACCOUNT:parameter/localmdm/db-password"},
        {"name": "JWT_SECRET", "valueFrom": "arn:aws:ssm:REGION:ACCOUNT:parameter/localmdm/jwt-secret"},
        {"name": "KEYCLOAK_CLIENT_SECRET", "valueFrom": "arn:aws:ssm:REGION:ACCOUNT:parameter/localmdm/keycloak-secret"},
        {"name": "DEP_ENCRYPTION_KEY", "valueFrom": "arn:aws:ssm:REGION:ACCOUNT:parameter/localmdm/dep-encryption-key"}
      ],
      "environment": [
        {"name": "ENVIRONMENT", "value": "production"},
        {"name": "DB_HOST", "value": "localmdm-primary.XXXXX.REGION.rds.amazonaws.com"},
        {"name": "DB_PORT", "value": "5432"},
        {"name": "DB_USER", "value": "localmdm"},
        {"name": "DB_NAME", "value": "localmdm"},
        {"name": "DB_READER_HOST", "value": "localmdm-replica.XXXXX.REGION.rds.amazonaws.com"},
        {"name": "DB_READER_PORT", "value": "5432"}
      ],
      "logConfiguration": {
        "logDriver": "awslogs",
        "options": {
          "awslogs-group": "/ecs/localmdm",
          "awslogs-region": "REGION",
          "awslogs-stream-prefix": "app"
        }
      }
    },
    {
      "name": "cloudwatch-agent",
      "image": "public.ecr.aws/cloudwatch-agent/cloudwatch-agent:latest",
      "essential": false,
      "portMappings": [],
      "secrets": [],
      "environment": [
        {"name": "CW_CONFIG_CONTENT", "value": "{\"metrics\":{\"namespace\":\"LocalMDM\",\"metrics_collected\":{\"prometheus\":{\"prometheus_config_path\":\"/opt/aws/amazon-cloudwatch-agent/etc/prometheus.yaml\",\"emf_processor\":{\"metric_declaration\":[{\"source_labels\":[\"job\"],\"label_matcher\":\"localmdm\",\"dimensions\":[[\"instance\"]],\"metric_selectors\":[\".*\"]}]}}}}}"}
      ],
      "mountPoints": [
        {
          "sourceVolume": "prometheus-config",
          "containerPath": "/opt/aws/amazon-cloudwatch-agent/etc"
        }
      ],
      "logConfiguration": {
        "logDriver": "awslogs",
        "options": {
          "awslogs-group": "/ecs/localmdm",
          "awslogs-region": "REGION",
          "awslogs-stream-prefix": "cw-agent"
        }
      }
    }
  ],
  "volumes": [
    {
      "name": "prometheus-config",
      "host": {}
    }
  ]
}
```

**CloudWatch Agent Prometheus config** (`prometheus.yaml` — baked into image or injected via init container):
```yaml
global:
  scrape_interval: 30s
scrape_configs:
  - job_name: localmdm
    static_configs:
      - targets: ["localhost:9090"]
```

### 2. NanoMDM Task Definition

NanoMDM is the Apple MDM protocol handler. It runs as a separate ECS service sharing the same RDS database.

```json
{
  "family": "nanomdm",
  "networkMode": "awsvpc",
  "requiresCompatibilities": ["FARGATE"],
  "cpu": "256",
  "memory": "512",
  "executionRoleArn": "arn:aws:iam::ACCOUNT:role/localmdm-execution",
  "taskRoleArn": "arn:aws:iam::ACCOUNT:role/localmdm-task",
  "containerDefinitions": [
    {
      "name": "nanomdm",
      "image": "ACCOUNT.dkr.ecr.REGION.amazonaws.com/nanomdm:latest",
      "essential": true,
      "portMappings": [
        {"containerPort": 9000, "protocol": "tcp"}
      ],
      "environment": [
        {"name": "NANOMDM_LISTEN", "value": ":9000"},
        {"name": "NANOMDM_STORAGE", "value": "pgsql"},
        {"name": "NANOMDM_STORAGE_DSN", "value": "postgres://nanomdm:PASSWORD@localmdm-primary.XXXXX.REGION.rds.amazonaws.com:5432/localmdm?sslmode=require"},
        {"name": "NANOMDM_WEBHOOK_URL", "value": "http://localmdm.localmdm-ns:8080"},
        {"name": "NANOMDM_API", "value": "nanomdm-api-key"}
      ],
      "logConfiguration": {
        "logDriver": "awslogs",
        "options": {
          "awslogs-group": "/ecs/nanomdm",
          "awslogs-region": "REGION",
          "awslogs-stream-prefix": "app"
        }
      }
    }
  ]
}
```

**ALB routing** (path-based):
- `/checkin`, `/mdm` → NanoMDM target group (port 9000) — Apple device check-ins and command responses
- All other paths → Local MDM target group (port 8080) — API, enrollment, dashboard

Local MDM sends commands to NanoMDM via its internal API (`nanomdm_url` in config), authenticated with `nanomdm_api_key`. NanoMDM pushes commands to devices via APNs and delivers responses back to Local MDM's `/checkin` and `/mdm` webhook endpoints.

### 3. ECS Service & ALB

```json
{
  "serviceName": "localmdm",
  "cluster": "localmdm-cluster",
  "taskDefinition": "localmdm",
  "desiredCount": 3,
  "launchType": "FARGATE",
  "networkConfiguration": {
    "awsvpcConfiguration": {
      "subnets": ["subnet-private-1a", "subnet-private-1b", "subnet-private-1c"],
      "securityGroups": ["sg-localmdm-tasks"],
      "assignPublicIp": "DISABLED"
    }
  },
  "loadBalancers": [
    {
      "targetGroupArn": "arn:aws:elasticloadbalancing:REGION:ACCOUNT:targetgroup/localmdm/XXXXX",
      "containerName": "localmdm",
      "containerPort": 8080
    }
  ],
  "deploymentConfiguration": {
    "maximumPercent": 200,
    "minimumHealthyPercent": 100,
    "deploymentCircuitBreaker": {
      "enable": true,
      "rollback": true
    }
  },
  "healthCheckGracePeriodSeconds": 60
}
```

**ALB Configuration**:
- **Listener**: HTTPS:443 → target group (ACM certificate attached)
- **Target group**: port 8080, health check on `/health`, healthy threshold 2, interval 15s
- **Security group**: allow 443 inbound from 0.0.0.0/0, allow 8080 from ALB SG only

### 3. Auto-Scaling

```json
{
  "ServiceNamespace": "ecs",
  "ResourceId": "service/localmdm-cluster/localmdm",
  "ScalableDimension": "ecs:service:DesiredCount",
  "MinCapacity": 2,
  "MaxCapacity": 10
}
```

**Scaling policies**:
- **CPU target tracking**: scale when average CPU > 70%
- **Request count**: scale when ALB requests per target > 1000/min
- **Scale-in cooldown**: 300s (prevent flapping)

### 4. RDS PostgreSQL

| Setting | Value |
|---------|-------|
| Engine | PostgreSQL 15+ |
| Instance class | db.t4g.medium (production) / db.t4g.micro (staging) |
| Multi-AZ | Yes (primary) |
| Read replica | 1 (maps to `DB_READER_HOST`) |
| Storage | gp3, 50GB, auto-scaling to 200GB |
| Backup retention | 7 days |
| Encryption | AES-256 (default KMS key) |
| Parameter group | `max_connections=200`, `shared_buffers=256MB` |

The Writer/Reader pool split from Sprint 4b maps directly:
- `DB_HOST` → RDS primary endpoint (writes + transactions)
- `DB_READER_HOST` → RDS reader endpoint (read queries)

### 5. Secrets Management (SSM Parameter Store)

All secrets stored as `SecureString` parameters:

| Parameter | Type | Description |
|-----------|------|-------------|
| `/localmdm/db-password` | SecureString | RDS master password |
| `/localmdm/jwt-secret` | SecureString | JWT signing key |
| `/localmdm/keycloak-secret` | SecureString | Keycloak client secret |
| `/localmdm/dep-encryption-key` | SecureString | DEP token encryption key |
| `/localmdm/nanomdm-api-key` | SecureString | NanoMDM API authentication key |

ECS task execution role needs `ssm:GetParameters` permission on these paths. Secrets are injected as environment variables at task launch — the Go app reads them via `os.Getenv()` (already implemented in `config.go`).

### 6. TLS (ACM + ALB)

- Request certificate in ACM for `mdm.example.com`
- DNS validation via Route 53 (auto-renewing, zero maintenance)
- Attach certificate to ALB HTTPS listener
- ALB terminates TLS; traffic to ECS tasks is HTTP on port 8080 within the VPC

### 7. CloudWatch Integration

**Logs**: ECS `awslogs` driver sends container stdout/stderr to CloudWatch Logs. The Go app uses structured JSON logging — CloudWatch Logs Insights can query by field.

**Metrics**: CloudWatch Agent sidecar scrapes Prometheus metrics from `localhost:9090` and publishes to CloudWatch Metrics under the `LocalMDM` namespace. This gives you:
- All existing Prometheus metrics (request rate, latency, error rate) in CloudWatch dashboards
- CloudWatch Alarms on any metric (e.g., error rate > 5%)
- No need to run a separate Prometheus server in production

**Dashboard**: CloudWatch dashboard with panels for:
- Request rate and latency (p50, p95, p99)
- Error rate by endpoint
- ECS task count and CPU/memory utilization
- RDS connections, read/write IOPS, replication lag
- ALB request count, 5xx rate, target response time

### 8. Networking

```
VPC (10.0.0.0/16)
├── Public subnets (10.0.1.0/24, 10.0.2.0/24, 10.0.3.0/24)
│   └── ALB (internet-facing)
├── Private subnets (10.0.10.0/24, 10.0.20.0/24, 10.0.30.0/24)
│   └── ECS tasks (no public IP)
└── Database subnets (10.0.100.0/24, 10.0.200.0/24)
    └── RDS (private, no internet access)
```

Security groups:
- **ALB SG**: inbound 443 from 0.0.0.0/0
- **ECS SG**: inbound 8080 from ALB SG only
- **RDS SG**: inbound 5432 from ECS SG only

---

## Implementation Tasks

### Task 1: Infrastructure (1 day)
- Create ECR repository, push Docker image
- Create ECS cluster (Fargate)
- Create task definition with app + CloudWatch Agent sidecar
- Create ECS service with ALB target group
- Configure auto-scaling policies

### Task 2: Database & Secrets (0.5 days)
- Create RDS PostgreSQL with read replica
- Store secrets in SSM Parameter Store
- Run migrations against RDS
- Verify Writer/Reader pool connectivity

### Task 3: Networking & TLS (0.5 days)
- Create VPC with public/private/database subnets
- Configure ALB with ACM certificate
- Set up security groups
- Configure Route 53 DNS

### Task 4: Monitoring & Operations (0.5 days)
- Verify CloudWatch Agent scrapes Prometheus metrics
- Create CloudWatch dashboard
- Set up alarms (error rate, latency, task health)
- Write production troubleshooting guide

### Task 5: Deployment Pipeline (0.5 days)
- ECR image build and push (GitHub Actions or CodePipeline)
- ECS service update (rolling deployment with circuit breaker)
- Database migration step (run before deploy)
- Rollback procedure documentation

---

## Zero-Downtime Deployment

ECS handles this natively with the deployment configuration above:
- `minimumHealthyPercent: 100` — never fewer tasks than desired count
- `maximumPercent: 200` — spin up new tasks before draining old ones
- `deploymentCircuitBreaker` — auto-rollback if new tasks fail health checks
- ALB drains connections from old tasks (deregistration delay: 30s)

**Deployment flow**:
1. Push new image to ECR
2. Update ECS service (new task definition revision)
3. ECS launches new tasks with new image
4. ALB health checks pass → new tasks receive traffic
5. Old tasks drain and stop
6. If health checks fail → circuit breaker rolls back automatically

---

## Cost Estimate

### Staging
| Resource | Spec | Monthly Cost |
|----------|------|-------------|
| ECS Fargate (localmdm) | 1 task, 0.25 vCPU, 0.5GB | ~$9 |
| ECS Fargate (nanomdm) | 1 task, 0.25 vCPU, 0.5GB | ~$9 |
| ECS Fargate (keycloak) | 1 task, 0.5 vCPU, 1GB | ~$18 |
| RDS | db.t4g.micro, single-AZ, no replica | ~$13 |
| ALB | minimal traffic | ~$16 |
| CloudWatch | logs + metrics | ~$5 |
| **Total** | | **~$70/mo** |

### Production
| Resource | Spec | Monthly Cost |
|----------|------|-------------|
| ECS Fargate (localmdm) | 3 tasks, 0.5 vCPU, 1GB | ~$55 |
| ECS Fargate (nanomdm) | 2 tasks, 0.25 vCPU, 0.5GB | ~$18 |
| ECS Fargate (keycloak) | 2 tasks, 1 vCPU, 2GB | ~$72 |
| RDS | db.t4g.medium, Multi-AZ + 1 replica | ~$140 |
| ALB | moderate traffic | ~$25 |
| CloudWatch | logs + metrics + dashboards | ~$15 |
| SSM | parameter reads | ~$0 |
| ACM | certificate | $0 |
| **Total** | | **~$325/mo** |

---

## Security Hardening (from SECURITY.md TODOs)

- **Dependency scanning**: `govulncheck` in CI pipeline
- **Image scanning**: ECR image scanning on push
- **Network segmentation**: VPC with private subnets, no public IPs on tasks
- **Secrets**: SSM Parameter Store with KMS encryption, never in task definition plaintext
- **IAM**: least-privilege task role and execution role
- **Security contact**: set up `security@localmdm.dev` for responsible disclosure

---

## Acceptance Criteria

- [ ] ECS service running with 3 tasks behind ALB
- [ ] Health checks pass (ALB target group healthy)
- [ ] Zero-downtime rolling deployment works
- [ ] Auto-scaling triggers on CPU threshold
- [ ] Circuit breaker rolls back failed deployments
- [ ] RDS primary + read replica connected (Writer/Reader pools)
- [ ] Secrets loaded from SSM Parameter Store
- [ ] TLS termination at ALB with ACM certificate
- [ ] CloudWatch Agent forwards Prometheus metrics to CloudWatch Metrics
- [ ] CloudWatch dashboard shows request rate, latency, error rate
- [ ] CloudWatch Alarms configured for error rate and task health

---

## Appendix: Kubernetes Deployment (Alternative)

For teams that prefer Kubernetes or already have a K8s cluster, the same application can be deployed with standard Kubernetes resources. The core concepts (health checks, rolling deploys, read replicas, secrets) are identical — only the orchestration layer differs.

### Deployment

```yaml
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
        envFrom:
        - configMapRef:
            name: localmdm-config
        - secretRef:
            name: localmdm-secrets
```

### ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: localmdm-config
data:
  ENVIRONMENT: "production"
  DB_HOST: "postgres-primary.default.svc.cluster.local"
  DB_PORT: "5432"
  DB_NAME: "localmdm"
  DB_READER_HOST: "postgres-replica.default.svc.cluster.local"
  DB_READER_PORT: "5432"
```

### Service, Ingress, HPA

```yaml
apiVersion: v1
kind: Service
metadata:
  name: localmdm
spec:
  type: ClusterIP
  ports:
  - port: 8080
    targetPort: 8080
  selector:
    app: localmdm
---
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
---
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
```

### Helm Chart

For repeated deployments, a Helm chart parameterizes the above manifests. Structure:

```
helm/localmdm/
├── Chart.yaml
├── values.yaml
├── values-production.yaml
├── templates/
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── ingress.yaml
│   ├── configmap.yaml
│   ├── hpa.yaml
│   └── pdb.yaml
```

See the Kubernetes documentation and Helm docs for details on managing chart releases.

---

## References

- [ECS Fargate Documentation](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/AWS_Fargate.html)
- [CloudWatch Agent with Prometheus](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/ContainerInsights-Prometheus.html)
- [S5-04: Deployment Guide](../sprint-5-ui-and-polish/S5-04-deployment.md)
- [S5-06: Observability](../sprint-5-ui-and-polish/S5-06-observability.md)
