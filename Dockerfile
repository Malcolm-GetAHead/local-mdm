# === Production build ===
FROM golang:1.26-alpine AS builder
RUN apk add --no-cache git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /localmdm ./cmd/server/
RUN CGO_ENABLED=0 go build -o /localmdm-cli ./cmd/cli/
RUN CGO_ENABLED=0 go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

FROM alpine:3.19 AS prod
RUN apk add --no-cache ca-certificates curl
COPY --from=builder /localmdm /app/localmdm
COPY --from=builder /localmdm-cli /app/localmdm-cli
COPY --from=builder /go/bin/migrate /usr/local/bin/migrate
COPY configs/ /app/configs/
COPY migrations/ /app/migrations/
RUN /app/localmdm-cli certs init --cert /app/certs/ca.crt --key /app/certs/ca.key
WORKDIR /app
EXPOSE 8080 9090
ENTRYPOINT ["/app/localmdm"]

# === Dev mode (hot reload + test tools) ===
FROM golang:1.26-alpine AS dev
RUN apk add --no-cache git curl postgresql-client gcc musl-dev
RUN go install github.com/air-verse/air@latest
RUN go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
RUN cd /tmp && git clone --depth 1 https://github.com/jessepeterson/mdmb.git && \
    cd mdmb && \
    # Patch: Load() doesn't restore OSVersion/BuildVersion/ProductName (upstream bug) \
    sed -i 's/device.MDMProfileIdentifier = BucketGetString(tx, "device_mdm_profile_id", udid)/device.MDMProfileIdentifier = BucketGetString(tx, "device_mdm_profile_id", udid)\n\t\tdevice.BuildVersion = BucketGetString(tx, "device_build_version", udid)\n\t\tdevice.OSVersion = BucketGetString(tx, "device_os_version", udid)\n\t\tdevice.ProductName = BucketGetString(tx, "device_product_name", udid)/' internal/device/storage.go && \
    go build -o /go/bin/mdmb ./cmd/mdmb/ && rm -rf /tmp/mdmb
WORKDIR /src
# Source mounted as volume at /src
EXPOSE 8080 9090
CMD ["air", "-c", ".air.toml"]
