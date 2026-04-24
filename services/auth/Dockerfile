FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy workspace files
COPY go.work go.work.sum ./
COPY shared/ ./shared/
COPY services/auth/ ./services/auth/

WORKDIR /app/services/auth
RUN go build -trimpath -ldflags="-s -w" -o /bin/auth-service ./cmd/server

# --- Runtime image ---
FROM gcr.io/distroless/static-debian12

COPY --from=builder /bin/auth-service /auth-service

EXPOSE 8081

USER nonroot:nonroot

ENTRYPOINT ["/auth-service"]
