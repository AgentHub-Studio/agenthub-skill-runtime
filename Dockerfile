FROM golang:1.25.12-alpine AS builder
RUN apk add --no-cache git ca-certificates tzdata
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -extldflags '-static'" \
    -o /build/bin/skillruntime \
    ./cmd/skillruntime

FROM scratch
ARG OCI_CREATED="unknown"
ARG OCI_REVISION="unknown"
ARG OCI_SOURCE="https://github.com/AgentHub-Studio/agenthub-skill-runtime"
ARG OCI_VERSION="local"
LABEL org.opencontainers.image.title="agenthub-skill-runtime" \
    org.opencontainers.image.description="AgentHub skill runtime service" \
    org.opencontainers.image.source="${OCI_SOURCE}" \
    org.opencontainers.image.revision="${OCI_REVISION}" \
    org.opencontainers.image.created="${OCI_CREATED}" \
    org.opencontainers.image.version="${OCI_VERSION}" \
    org.opencontainers.image.vendor="AgentHub Studio" \
    org.opencontainers.image.licenses="Proprietary"
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /build/bin/skillruntime /skillruntime
USER 65532:65532
EXPOSE 8083
ENTRYPOINT ["/skillruntime"]
