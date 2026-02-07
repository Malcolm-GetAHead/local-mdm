# SCEP

> Source: [github.com/micromdm/scep](https://github.com/micromdm/scep)

SCEP is a Simple Certificate Enrollment Protocol server and client.

## Role in Local MDM

SCEP provides the certificate enrollment infrastructure required by Apple MDM. Devices use SCEP to obtain identity certificates during MDM enrollment. NanoMDM validates these certificates to authenticate devices.

**Note:** The upstream project notes that the included SCEP server/CA is basic and lacks some security features. For production, consider [smallstep/certificates](https://github.com/smallstep/certificates) as an alternative.

## Server Usage

```bash
# Create a new CA
./scepserver ca -init

# Start server
./scepserver -depot depot -port 2016 -challenge=secret
```

### Key Server Flags

| Flag | Default | Description |
|---|---|---|
| `-depot` | `depot` | Path to CA folder (must contain `ca.pem` and `ca.key`) |
| `-port` | `8080` | Listen port |
| `-challenge` | — | Challenge password for enrollment |
| `-allowrenew` | `14` | Days before expiry to allow renewal (0 = always) |
| `-crtvalid` | `365` | Validity for new client certificates in days |
| `-csrverifierexec` | — | External CSR verification script |
| `-capass` | — | Password for ca.key |
| `-debug` | false | Debug logging |

### CA Sub-command

```bash
./scepserver ca -init -key-password secret -keySize 4096 -years 10
```

| Flag | Default | Description |
|---|---|---|
| `-common_name` | `MICROMDM SCEP CA` | CA certificate CN |
| `-organization` | `scep-ca` | Organization |
| `-country` | `US` | Country code |
| `-keySize` | `4096` | RSA key size |
| `-years` | `10` | CA validity in years |

## Client Usage

```bash
./scepclient -private-key client.key -server-url=http://127.0.0.1:2016/scep -challenge=secret
```

### Key Client Flags

| Flag | Default | Description |
|---|---|---|
| `-server-url` | — | SCEP server URL (include `/scep` path) |
| `-challenge` | — | Challenge password |
| `-private-key` | — | Private key path (created if missing) |
| `-cn` | `scepclient` | Common name for certificate |
| `-organization` | `scep-client` | Organization |
| `-keySize` | `2048` | RSA key size |
| `-ca-fingerprint` | — | SHA-256 CA fingerprint (for NDES) |

## HTTP Endpoint

The server provides a single endpoint at `/scep` supporting standard SCEP PKIOperation/Message parameters.

## Go Library

The SCEP package can be imported into Go projects. Key interfaces:

- **`Depot`** — certificate storage abstraction
- **`CSRSigner`** — certificate signing abstraction

This allows swapping the CA signer or using SCEP as a proxy. See [scepserver.go](https://github.com/micromdm/scep/blob/main/cmd/scepserver/scepserver.go) for an integration example.

## Docker

```bash
CGO_ENABLED=0 make docker
docker build -t micromdm/scep:latest .
docker run -it --rm -v /path/to/ca:/depot micromdm/scep:latest ca -init
docker run -it --rm -v /path/to/ca:/depot -p 8080:8080 micromdm/scep:latest
```
