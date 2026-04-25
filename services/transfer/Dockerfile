FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.work go.work.sum ./
COPY shared/ ./shared/
COPY services/ ./services/

WORKDIR /app/services/transfer
RUN go build -trimpath -ldflags="-s -w" -o /bin/transfer-service ./cmd/server

FROM gcr.io/distroless/static-debian12

COPY --from=builder /bin/transfer-service /transfer-service

EXPOSE 8082

USER nonroot:nonroot

ENTRYPOINT ["/transfer-service"]
