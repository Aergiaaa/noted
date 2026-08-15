FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/noted ./cmd

FROM alpine:3.22
RUN apk add --no-cache ca-certificates postgresql-client
RUN adduser -D -u 10001 noted
WORKDIR /app
COPY --from=builder /out/noted /usr/local/bin/noted
COPY --from=builder /src/static ./static
COPY internal/database/migrations /migrations
COPY entrypoint.sh /entrypoint.sh
USER noted
ENTRYPOINT ["/entrypoint.sh"]
CMD ["noted"]