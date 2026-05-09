FROM golang:1.26.3-alpine AS builder

WORKDIR /app

COPY go.work go.work.sum ./
COPY vendor/ ./vendor/
COPY shared/ ./shared/
COPY services/ ./services/

WORKDIR /app/services/admin
RUN go build -mod=vendor -trimpath -ldflags="-s -w" -o /bin/admin-service ./cmd/server

FROM gcr.io/distroless/static-debian12

COPY --from=builder /bin/admin-service /admin-service

EXPOSE 8087

USER nonroot:nonroot

ENTRYPOINT ["/admin-service"]
