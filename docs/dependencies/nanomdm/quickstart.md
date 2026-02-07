# NanoMDM Quickstart Guide

> Source: [github.com/micromdm/nanomdm/docs/quickstart.md](https://github.com/micromdm/nanomdm/blob/main/docs/quickstart.md)

This quickstart guide is intended to quickly get a functioning NanoMDM instance up and running. This guide will use [ngrok](https://ngrok.com/) for easy setup and configuration of both public access & TLS.

**Warning:** ngrok actively proxies live internet traffic to your computer. This guide is intended to get NanoMDM *working* and *does not represent best practices*.

## Requirements

- An MDM push certificate and private key
- A posix-ish computer with standard command-line tools: `cat`, `curl`, Python 3, etc.
- Direct internet access

## SCEP Server

If you don't already have a SCEP server, set one up using MicroMDM's SCEP server:

```bash
$ mkdir scep && cd scep
$ curl -RLO https://github.com/micromdm/scep/releases/download/v2.1.0/scepserver-darwin-amd64-v2.1.0.zip
$ unzip scepserver-darwin-amd64-v2.1.0.zip
$ ./scepserver-darwin-amd64 ca -init
$ ./scepserver-darwin-amd64 -allowrenew 0 -challenge nanomdm -debug
```

## NanoMDM Setup

### Retrieve SCEP CA certificate

```bash
$ curl 'https://<scep-url>/scep?operation=GetCACert' | openssl x509 -inform DER > ca.pem
```

### Run NanoMDM

```bash
$ ./nanomdm-darwin-amd64 -ca ca.pem -api nanomdm -debug
```

### Upload Push Certificate

```bash
$ cat /path/to/push.pem /path/to/push.key | curl -T - -u nanomdm:nanomdm 'http://127.0.0.1:9000/v1/pushcert'
```

Returns the APNS topic:
```json
{ "topic": "com.apple.mgmt.External.e3b8ceac-1f18-2c8e-8a63-dd17d99435d9" }
```

## Enrollment Profile

Configure your enrollment profile with:
- `URL` (SCEP payload): your SCEP server URL + `/scep`
- `Challenge` (SCEP payload): your SCEP challenge
- `ServerURL` (MDM payload): your NanoMDM URL + `/mdm`
- `Topic` (MDM payload): the topic from push cert upload

## Send Push Notification

```bash
$ curl -u nanomdm:nanomdm 'http://127.0.0.1:9000/v1/push/<enrollment-id>'
```

## Send a Command

```bash
$ ./tools/cmdr.py SecurityInfo | curl -T - -u nanomdm:nanomdm 'http://127.0.0.1:9000/v1/enqueue/<enrollment-id>'
```
