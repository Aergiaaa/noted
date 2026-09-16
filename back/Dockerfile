# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Static binary for distroless runtime
RUN CGO_ENABLED=0 go build -trimpath -o /out/server ./cmd/server

# Runtime stage: minimal non-root with wget for healthcheck.
# NOTE: DEPLOYMENT.md says distroless, but distroless/static ships no
# shell/wget so `wget /healthz` healthchecks can never pass there.
# Alpine + non-root user keeps the intent (small, non-root, wget-able);
# revisit in F11 hardening if strict distroless is wanted.
FROM alpine:3.21
RUN apk add --no-cache ca-certificates wget \
  && adduser -D -u 65532 appuser
COPY --from=build /out/server /server
EXPOSE 8080
USER appuser
ENTRYPOINT ["/server"]
