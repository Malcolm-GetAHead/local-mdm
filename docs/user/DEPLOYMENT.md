# Deployment Guide

## Development (Docker Compose)

The project includes a `docker-compose.yml` with PostgreSQL, Keycloak, and Adminer.

### Quick Start

```bash
# Start services
make docker-up

# Wait for Keycloak to be healthy (~45s)
sleep 45

# Run database migrations
make migrate-up

# Start the server
make run
```

### Services

| Service    | Port  | Purpose                        |
|------------|-------|--------------------------------|
| PostgreSQL | 5432  | Application + Keycloak database |
| Keycloak   | 8180  | OIDC identity provider          |
| Adminer    | 8081  | Database admin UI               |
| Local MDM  | 8080  | Application API                 |
| Metrics    | 9090  | Prometheus metrics (localhost)   |

### docker-compose.yml

The included compose file provisions:

- **postgres** — PostgreSQL 15 Alpine with two databases (`localmdm`, `keycloak`) created via init script. Data persisted in a named volume.
- **keycloak** — Keycloak 23.0 in dev mode with realm auto-import from `docker/keycloak/realm-export.json`.
- **adminer** — Database browser at `http://localhost:8081`.

### Configuration

Copy the example config and set required secrets via environment variables:

```bash
cp configs/config.example.yaml configs/config.yaml

# Required — these override YAML values
export DB_PASSWORD="your-db-password"
export JWT_SECRET="minimum-32-character-secret-here"
export KEYCLOAK_CLIENT_SECRET="your-keycloak-secret"
```

---

## Environment Variables

All environment variables override their YAML config equivalents.

| Variable                 | Required | Description                              |
|--------------------------|----------|------------------------------------------|
| `ENVIRONMENT`            | No       | `development`, `staging`, `production`   |
| `DB_HOST`                | No       | Database host (default: `localhost`)      |
| `DB_PORT`                | No       | Database port (default: `5432`)           |
| `DB_USER`                | No       | Database user (default: `postgres`)       |
| `DB_PASSWORD`            | Yes      | Database password                         |
| `DB_NAME`                | No       | Database name (default: `localmdm`)       |
| `DB_READER_HOST`         | No       | Read replica host (falls back to writer)  |
| `DB_READER_PORT`         | No       | Read replica port (falls back to writer)  |
| `JWT_SECRET`             | Yes      | JWT signing secret (min 32 chars)         |
| `KEYCLOAK_CLIENT_SECRET` | Yes      | Keycloak OIDC client secret               |
| `DEP_ENCRYPTION_KEY`     | No       | Encryption key for DEP token storage      |

---

## Health Checks

Two endpoints are available without authentication:

### GET /health (Liveness)

Returns overall system status. Returns 200 if the database is reachable, 503 otherwise. Keycloak degradation does not cause 503.

```json
{
  "data": {
    "status": "healthy",
    "version": "1.0.0",
    "checks": {
      "database": "healthy",
      "keycloak": "healthy"
    },
    "timestamp": "2026-04-21T16:00:00Z"
  }
}
```

### GET /health/ready (Readiness)

Returns per-dependency status with latency. Use this for load balancer readiness probes.

```json
{
  "ready": true,
  "checks": {
    "database": { "status": "healthy", "latency": "1.2ms" },
    "keycloak": { "status": "healthy", "latency": "15ms" }
  },
  "timestamp": "2026-04-21T16:00:00Z"
}
```

---

## Production (AWS ECS Fargate)

### Architecture

```
Internet → ALB (TLS/ACM) → ECS Fargate Tasks
                              ├── localmdm (API)
                              ├── nanomdm (Apple MDM)
                              └── keycloak (OIDC)
                            ↓
                     RDS PostgreSQL (primary + read replica)
```

### ECS Services

| Service      | Tasks | ALB Path Routing                  |
|--------------|-------|-----------------------------------|
| localmdm     | 2+    | Default (all API traffic)         |
| nanomdm      | 1+    | `/checkin`, `/mdm`                |
| keycloak     | 1+    | `/auth/*`                         |

### RDS PostgreSQL

- Primary instance → Writer pool (all writes, transactions)
- Read replica → Reader pool (read queries outside transactions)
- Both localmdm pools configured via env vars:

```bash
# Writer (primary)
DB_HOST=localmdm-primary.xxxxx.us-east-1.rds.amazonaws.com
DB_PORT=5432

# Reader (replica) — optional, falls back to writer
DB_READER_HOST=localmdm-replica.xxxxx.us-east-1.rds.amazonaws.com
DB_READER_PORT=5432
```

### Secrets (SSM Parameter Store)

Store all secrets in SSM and inject as environment variables in ECS task definitions:

```
/localmdm/prod/db-password          → DB_PASSWORD
/localmdm/prod/jwt-secret           → JWT_SECRET
/localmdm/prod/keycloak-secret      → KEYCLOAK_CLIENT_SECRET
/localmdm/prod/dep-encryption-key   → DEP_ENCRYPTION_KEY
/localmdm/prod/nanomdm-api-key      → NANOMDM_API_KEY
```

ECS task definition secret reference:

```json
{
  "secrets": [
    {
      "name": "DB_PASSWORD",
      "valueFrom": "arn:aws:ssm:us-east-1:123456789:parameter/localmdm/prod/db-password"
    },
    {
      "name": "JWT_SECRET",
      "valueFrom": "arn:aws:ssm:us-east-1:123456789:parameter/localmdm/prod/jwt-secret"
    }
  ]
}
```

### ALB Configuration

- TLS termination via ACM certificate (free, auto-renewing)
- Health check: `GET /health/ready` on port 8080, interval 30s, threshold 3
- Stickiness: not required (stateless design)
- WAF: attach AWS WAF with rate-based rules for production rate limiting

### ECS Task Definition (localmdm)

```json
{
  "family": "localmdm",
  "cpu": "512",
  "memory": "1024",
  "networkMode": "awsvpc",
  "containerDefinitions": [
    {
      "name": "localmdm",
      "image": "your-ecr-repo/localmdm:latest",
      "portMappings": [{ "containerPort": 8080 }],
      "environment": [
        { "name": "ENVIRONMENT", "value": "production" },
        { "name": "DB_HOST", "value": "localmdm-primary.xxxxx.rds.amazonaws.com" },
        { "name": "DB_NAME", "value": "localmdm" }
      ],
      "secrets": [
        { "name": "DB_PASSWORD", "valueFrom": "/localmdm/prod/db-password" },
        { "name": "JWT_SECRET", "valueFrom": "/localmdm/prod/jwt-secret" },
        { "name": "KEYCLOAK_CLIENT_SECRET", "valueFrom": "/localmdm/prod/keycloak-secret" }
      ],
      "logConfiguration": {
        "logDriver": "awslogs",
        "options": {
          "awslogs-group": "/ecs/localmdm",
          "awslogs-region": "us-east-1",
          "awslogs-stream-prefix": "localmdm"
        }
      },
      "healthCheck": {
        "command": ["CMD-SHELL", "curl -f http://localhost:8080/health || exit 1"],
        "interval": 30,
        "timeout": 5,
        "retries": 3
      }
    }
  ]
}
```

---

## Reverse Proxy (nginx)

For self-hosted deployments with TLS termination:

```nginx
upstream localmdm {
    server 127.0.0.1:8080;
}

server {
    listen 443 ssl http2;
    server_name mdm.example.com;

    ssl_certificate     /etc/nginx/ssl/mdm.example.com.crt;
    ssl_certificate_key /etc/nginx/ssl/mdm.example.com.key;
    ssl_protocols       TLSv1.2 TLSv1.3;
    ssl_ciphers         HIGH:!aNULL:!MD5;

    # Health check (no auth)
    location /health {
        proxy_pass http://localmdm;
    }

    # API
    location / {
        proxy_pass http://localmdm;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        proxy_read_timeout 30s;
        proxy_send_timeout 30s;
    }

    # Keycloak (if co-hosted)
    location /auth/ {
        proxy_pass http://127.0.0.1:8180/;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

server {
    listen 80;
    server_name mdm.example.com;
    return 301 https://$host$request_uri;
}
```

---

## Configuration Reference

Full YAML config with defaults — see `configs/config.example.yaml`:

```yaml
environment: "production"

server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 120s
  request_timeout: 30s
  rate_limit:
    enabled: true
    requests_per_min: 100

database:
  host: "localhost"
  port: 5432
  user: "postgres"
  password: ""           # Use DB_PASSWORD env var
  database: "localmdm"
  sslmode: "require"     # Use "require" in production
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: 5m
  # reader:              # Optional read replica
  #   host: "replica.example.com"

auth:
  jwt_secret: ""         # Use JWT_SECRET env var (min 32 chars)
  access_token_duration: 1h
  refresh_token_duration: 168h

keycloak:
  url: "https://keycloak.example.com"
  realm: "localmdm"
  client_id: "localmdm-api"
  client_secret: ""      # Use KEYCLOAK_CLIENT_SECRET env var

certificates:
  ca_cert_path: "./certs/ca.crt"
  ca_key_path: "./certs/ca.key"
  device_cert_validity: 8760h
  scep_challenge_ttl: 10m

logging:
  level: "info"
  format: "json"
  output: "stdout"
```
