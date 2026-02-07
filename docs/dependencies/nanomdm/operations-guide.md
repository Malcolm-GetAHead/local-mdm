# NanoMDM Operations Guide

> Source: [github.com/micromdm/nanomdm/docs/operations-guide.md](https://github.com/micromdm/nanomdm/blob/main/docs/operations-guide.md)

## Enrollment IDs

NanoMDM collapses various Apple MDM identifiers into a single "enrollment ID":

| Type | Platform | ID Format | Example |
|---|---|---|---|
| Device | macOS | `UUID` | `470E005B-17C1-4537-BBB3-0EBC340D432A` |
| User | macOS | `UUID:UUID` | `470E005B-...:F151140B-...` |
| Device | iOS | `UUID` | `8b3b8ba3783e9ade1dae4fbb944ab3afc0ce5b69` |
| User Enrollment | iOS | `UUID` | `b318edb72b556059a013368e3150050c5f74a2c6` |
| Shared iPad | iOS | `UUID:ShortName` | `68656c6c6f...:appleid@example.com` |

## Key Command Line Flags

| Flag | Env Var | Default | Description |
|---|---|---|---|
| `-api` | `NANOMDM_API` | — | API key (HTTP Basic auth, username "nanomdm") |
| `-ca` | `NANOMDM_CA` | — | Path to PEM CA cert(s) for enrollment validation |
| `-listen` | `NANOMDM_LISTEN` | `:9000` | HTTP listen address |
| `-storage` | `NANOMDM_STORAGE` | `filekv` | Storage backend name |
| `-storage-dsn` | `NANOMDM_STORAGE_DSN` | `dbkv` | Storage data source name |
| `-debug` | `NANOMDM_DEBUG` | false | Enable debug logging |
| `-checkin` | `NANOMDM_CHECKIN` | false | Separate `/checkin` endpoint |
| `-webhook-url` | `NANOMDM_WEBHOOK_URL` | — | Webhook callback URL |
| `-dm` | `NANOMDM_DM` | — | Declarative Management URL |
| `-migration` | `NANOMDM_MIGRATION` | false | Enable migration endpoint |
| `-cert-header` | `NANOMDM_CERT_HEADER` | — | HTTP header for TLS client cert |
| `-disable-mdm` | `NANOMDM_DISABLE_MDM` | false | API-only mode |

## Storage Backends

### PostgreSQL (our choice)

```bash
-storage pgsql -storage-dsn postgres://user:pass@localhost:5432/nanomdm
```

Requires PostgreSQL 9.5+. Supports `delete=1` option to clean up after command responses.

### MySQL

```bash
-storage mysql -storage-dsn nanomdm:nanomdm/mymdmdb
```

Requires MySQL 8.0.19+.

### File / FileKV / In-Memory

Available for development/testing. See upstream docs for details.

## HTTP Endpoints & APIs

### MDM Endpoints

| Endpoint | Method | Description |
|---|---|---|
| `/mdm` | PUT | Primary MDM endpoint (commands, results, check-ins) |
| `/checkin` | PUT | Separate check-in endpoint (if `-checkin` enabled) |

### API Endpoints

| Endpoint | Method | Auth | Description |
|---|---|---|---|
| `/v1/pushcert` | PUT | Basic | Upload APNs push certificate (PEM cert+key body) |
| `/v1/push/{id}` | GET | Basic | Send APNs push to enrollment(s), comma-separated |
| `/v1/enqueue/{id}` | PUT | Basic | Queue command (raw Plist body), `?nopush=1` to skip push |
| `/migration` | PUT | Basic | Migration endpoint (if `-migration` enabled) |
| `/version` | GET | — | Server version |
| `/v1/escrowkeyunlock` | POST | Basic | Activation Lock unlock (form-encoded) |
| `/authproxy/` | * | MDM cert | Reverse proxy with MDM authentication |

### Push Example

```bash
curl -u nanomdm:apikey 'http://127.0.0.1:9000/v1/push/UUID1,UUID2'
```

### Enqueue Example

```bash
./cmdr.py SecurityInfo | curl -T - -u nanomdm:apikey 'http://127.0.0.1:9000/v1/enqueue/UUID1'
```

Response:
```json
{
  "status": { "UUID1": { "push_result": "..." } },
  "command_uuid": "...",
  "request_type": "SecurityInfo"
}
```

## Webhook

When `-webhook-url` is set, NanoMDM sends HTTP POST callbacks for MDM events. Topics:

- `mdm.Authenticate`
- `mdm.TokenUpdate`
- `mdm.CheckOut`
- `mdm.Connect`
- `mdm.UserAuthenticate`
- `mdm.SetBootstrapToken` / `mdm.GetBootstrapToken`
- `mdm.DeclarativeManagement`
- `mdm.GetToken`

Compatible with MicroMDM webhook format. See [webhook-event.json](webhook-event.json) for the full JSON schema.
