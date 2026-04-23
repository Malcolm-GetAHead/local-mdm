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
COPY internal/api/certs/ /app/certs/
WORKDIR /app
EXPOSE 8080 9090
ENTRYPOINT ["/app/localmdm"]

# === Dev mode (hot reload + test tools) ===
FROM golang:1.26-alpine AS dev
RUN apk add --no-cache git curl postgresql-client gcc musl-dev
RUN go install github.com/air-verse/air@latest
RUN go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
RUN go install github.com/jessepeterson/mdmb/cmd/mdmb@latest || \
    (cd /tmp && git clone https://github.com/jessepeterson/mdmb.git && cd mdmb && go build -o /go/bin/mdmb ./cmd/mdmb/)
WORKDIR /src
# Source mounted as volume at /src
EXPOSE 8080 9090
CMD ["air", "-c", ".air.toml"]
