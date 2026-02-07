# NanoDEP Quick Start Guide

> Source: [github.com/micromdm/nanodep/docs/quickstart.md](https://github.com/micromdm/nanodep/blob/main/docs/quickstart.md)

## Requirements

- Apple Business Manager (ABM) / Apple School Manager (ASM) account with Device Management permissions
- Devices already present in ABM/ASM
- `curl`, `jq`, and a shell interpreter
- Outbound internet access to Apple's DEP APIs

## Step-by-Step

### 1. Start depserver

```bash
$ ./depserver -api supersecret
# Listening on :9001
```

### 2. Setup environment

```bash
export BASE_URL='http://[::1]:9001'
export APIKEY=supersecret
export DEP_NAME=mdmserver1
```

### 3. Generate DEP token public key

```bash
$ ./tools/cfg-get-cert.sh > $DEP_NAME.pem
```

### 4. Upload public key to ABM/ASM portal

Upload the `.pem` file to your MDM server configuration in the Apple portal.

### 5. Download & decrypt tokens

Download the `.p7m` token file from the portal, then:

```bash
$ ./tools/cfg-decrypt-tokens.sh ~/Downloads/mdmserver1_Token_*.p7m
```

### 6. Verify connectivity

```bash
$ ./tools/dep-account-detail.sh
```

### 7. Define a DEP profile and assign devices

```bash
$ ./tools/dep-define-profile.sh ./dep-profile.json
```

### 8. Auto-assign with depsyncer

```bash
$ ./tools/cfg-set-assigner.sh <profile-uuid>
$ ./depsyncer mdmserver1
```

**Note:** Tokens must be renewed yearly or when Apple T&C are updated.
