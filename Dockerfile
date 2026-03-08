# ─── Build Stage ──────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /app
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build coordinator
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/coordinator ./cmd/coordinator

# Build client
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/client ./cmd/client

# ─── Coordinator Target ───────────────────────────────────────────────────────
FROM alpine:3.19 AS coordinator

RUN apk add --no-cache ca-certificates
WORKDIR /app

COPY --from=builder /bin/coordinator /app/coordinator
COPY web/ /app/web/

EXPOSE 8080
ENTRYPOINT ["/app/coordinator"]

# ─── Client Target ─────────────────────────────────────────────────────────────
FROM alpine:3.19 AS client

RUN apk add --no-cache ca-certificates
WORKDIR /app

COPY --from=builder /bin/client /app/client

ENTRYPOINT ["/app/client"]
