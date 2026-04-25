FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.work go.work.sum ./
COPY vendor/ ./vendor/
COPY shared/ ./shared/
COPY services/ ./services/

WORKDIR /app/services/upload
RUN go build -mod=vendor -trimpath -ldflags="-s -w" -o /bin/upload-service ./cmd/server

FROM gcr.io/distroless/static-debian12

COPY --from=builder /bin/upload-service /upload-service

EXPOSE 8084

USER nonroot:nonroot

ENTRYPOINT ["/upload-service"]
