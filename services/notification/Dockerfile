FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.work go.work.sum ./
COPY vendor/ ./vendor/
COPY shared/ ./shared/
COPY services/ ./services/

WORKDIR /app/services/notification
RUN go build -mod=vendor -trimpath -ldflags="-s -w" -o /bin/notification-service ./cmd/server

FROM gcr.io/distroless/static-debian12

COPY --from=builder /bin/notification-service /notification-service

EXPOSE 8085

USER nonroot:nonroot

ENTRYPOINT ["/notification-service"]
