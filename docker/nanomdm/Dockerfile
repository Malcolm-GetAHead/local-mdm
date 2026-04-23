FROM golang:1.26-alpine AS builder
RUN apk add --no-cache git
RUN git clone --branch v0.9.0 --depth 1 https://github.com/micromdm/nanomdm.git /src
WORKDIR /src
RUN CGO_ENABLED=0 go build -o /nanomdm ./cmd/nanomdm

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /nanomdm /app/nanomdm
EXPOSE 9000
ENTRYPOINT ["/app/nanomdm"]
