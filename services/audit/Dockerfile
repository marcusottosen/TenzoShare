FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.work go.work.sum ./
COPY vendor/ ./vendor/
COPY shared/ ./shared/
COPY services/ ./services/

WORKDIR /app/services/audit
RUN go build -mod=vendor -trimpath -ldflags="-s -w" -o /bin/audit-service ./cmd/server

FROM gcr.io/distroless/static-debian12

COPY --from=builder /bin/audit-service /audit-service

EXPOSE 8086

USER nonroot:nonroot

ENTRYPOINT ["/audit-service"]
