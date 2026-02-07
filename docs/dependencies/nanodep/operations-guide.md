# NanoDEP Operations Guide

> Source: [github.com/micromdm/nanodep/docs/operations-guide.md](https://github.com/micromdm/nanodep/blob/main/docs/operations-guide.md)

## DEP Names

NanoDEP supports multiple DEP "MDM server" configurations, each referenced by an arbitrary name string. Avoid forward-slashes, spaces, or URL-unfriendly characters in names.

## depserver

### Key Command Line Flags

| Flag | Env Var | Default | Description |
|---|---|---|---|
| `-api` | `NANODEP_API` | — | API key (HTTP Basic, username "depserver") |
| `-listen` | `NANODEP_LISTEN` | `:9001` | HTTP listen address |
| `-storage` | `NANODEP_STORAGE` | `filekv` | Storage backend |
| `-storage-dsn` | `NANODEP_STORAGE_DSN` | — | Storage connection string |
| `-debug` | `NANODEP_DEBUG` | false | Debug logging |

### Storage Backends

**PostgreSQL** (our choice):
```bash
-storage pgsql -storage-dsn postgres://user:pass@localhost:5432/nanodep
```

Also supports: `filekv` (default), `mysql`, `inmem`.

### API Endpoints

| Endpoint | Method | Description |
|---|---|---|
| `/version` | GET | Server version |
| `/v1/dep_names` | GET | Query configured DEP names |
| `/v1/tokenpki/{name}` | GET | Generate keypair, return certificate |
| `/v1/tokenpki/{name}` | PUT | Decrypt and store OAuth tokens from Apple portal |
| `/v1/tokens/{name}` | GET/PUT | Raw OAuth token management |
| `/v1/assigner/{name}` | GET/PUT | Get/set auto-assigner profile UUID |
| `/v1/config/{name}` | GET/PUT | DEP name configuration (base URL) |
| `/v1/maidjwt/{name}` | GET | Generate Managed Apple ID JWT |
| `/v1/bypasscode` | GET | Generate/decode Activation Lock Bypass Code |

### Reverse Proxy

Access Apple DEP APIs transparently via:
```
/proxy/{name}/endpoint
```

Translates to `https://mdmenrollment.apple.com/endpoint` with automatic session management.

Example:
```bash
curl -u depserver:apikey 'http://[::1]:9001/proxy/mdmserver1/account'
```

## depsyncer

Continuously syncs devices from Apple DEP and optionally auto-assigns profiles.

### Key Flags

| Flag | Default | Description |
|---|---|---|
| `-duration` | `1800` | Seconds between syncs (0 = single sync then exit) |
| `-limit` | `0` | Max devices per fetch (0 = server default of 100) |
| `-webhook-url` | — | URL for sync result webhooks |
| `-debug` | false | Debug logging |

### Usage

```bash
# Continuous sync
$ ./depsyncer mdmserver1

# Single sync
$ ./depsyncer -duration 0 mdmserver1

# Multiple DEP names
$ ./depsyncer -debug -limit 200 mdmserver1 mdmserver2
```

### Webhook Data

Sends HTTP POST with JSON body:
```json
{
  "topic": "dep.SyncDevices",
  "device_response_event": {
    "dep_name": "mdmserver1",
    "device_response": {
      "cursor": "...",
      "more_to_follow": false,
      "devices": [
        { "serial_number": "...", "op_type": "added" }
      ]
    }
  }
}
```

## Shell Script Tools

All scripts require `BASE_URL`, `APIKEY`, and `DEP_NAME` environment variables.

| Script | Purpose |
|---|---|
| `cfg-get-cert.sh` | Generate keypair, retrieve public cert for Apple portal |
| `cfg-decrypt-tokens.sh` | Upload encrypted tokens from Apple, decrypt and store |
| `cfg-set-assigner.sh` | Set auto-assigner profile UUID |
| `dep-account-detail.sh` | Get DEP account details |
| `dep-define-profile.sh` | Upload a DEP profile JSON |
| `dep-assign-profile.sh` | Assign profile UUID to serial number(s) |
| `dep-device-details.sh` | Get device details by serial |
| `dep-get-profile.sh` | Get profile by UUID |
| `dep-remove-profile.sh` | Remove profile assignment from serial(s) |
| `dep-activation-lock.sh` | Enable Activation Lock on a device |
