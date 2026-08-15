FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/noted ./cmd
RUN CGO_ENABLED=0 go build -o /out/migrate ./cmd/migrate

FROM alpine:3.22
RUN apk add --no-cache ca-certificates
RUN adduser -D -u 10001 noted
WORKDIR /app
COPY --from=builder /out/noted /usr/local/bin/noted
COPY --from=builder /out/migrate /usr/local/bin/migrate
COPY --from=builder /src/static ./static
USER noted
CMD ["noted"]