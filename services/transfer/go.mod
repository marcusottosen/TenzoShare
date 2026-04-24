module github.com/tenzoshare/tenzoshare/services/transfer

go 1.26

require (
	github.com/gofiber/fiber/v3 v3.1.0
	github.com/jackc/pgx/v5 v5.9.2
	github.com/nats-io/nats.go v1.51.0
	github.com/golang-jwt/jwt/v5 v5.3.1
	go.uber.org/zap v1.27.1
	connectrpc.com/connect v1.19.2
	google.golang.org/protobuf v1.36.11
	github.com/tenzoshare/tenzoshare/shared v0.0.0
)

replace github.com/tenzoshare/tenzoshare/shared => ../../shared
