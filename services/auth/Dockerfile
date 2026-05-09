FROM golang:1.26.3-alpine AS builder

WORKDIR /app

COPY go.work go.work.sum ./
COPY vendor/ ./vendor/
COPY shared/ ./shared/
COPY services/ ./services/

WORKDIR /app/services/auth
RUN go build -mod=vendor -trimpath -ldflags="-s -w" -o /bin/auth-service ./cmd/server

# --- Runtime image ---
FROM gcr.io/distroless/static-debian12

COPY --from=builder /bin/auth-service /auth-service

EXPOSE 8081

USER nonroot:nonroot

ENTRYPOINT ["/auth-service"]
