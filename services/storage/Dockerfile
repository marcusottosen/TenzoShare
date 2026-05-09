FROM golang:1.26.3-alpine AS builder

WORKDIR /app

COPY go.work go.work.sum ./
COPY vendor/ ./vendor/
COPY shared/ ./shared/
COPY services/ ./services/

WORKDIR /app/services/storage
RUN go build -mod=vendor -trimpath -ldflags="-s -w" -o /bin/storage-service ./cmd/server

FROM gcr.io/distroless/static-debian12

COPY --from=builder /bin/storage-service /storage-service

EXPOSE 8083

USER nonroot:nonroot

ENTRYPOINT ["/storage-service"]
