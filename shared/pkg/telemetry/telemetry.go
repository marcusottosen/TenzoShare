// Package telemetry exposes /health and /metrics endpoints for every
// TenzoShare backend service.
//
// Usage (in each service main.go, before auth middleware):
//
//	telemetry.Register(app, "auth")
package telemetry

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var startTime = time.Now()

// registry is a per-process Prometheus registry that includes the default
// Go runtime and process collectors.
var registry *prometheus.Registry

func init() {
	registry = prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
}

// Register mounts GET /health and GET /metrics on the Fiber app.
// Call this once per service, before any authentication middleware so the
// endpoints are reachable by health-check scrapers without credentials.
func Register(app *fiber.App, serviceName string) {
	app.Get("/health", healthHandler(serviceName))
	// Fiber v3 natively adapts net/http handlers — no wrapper needed.
	app.Get("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
}

func healthHandler(serviceName string) fiber.Handler {
	return func(c fiber.Ctx) error {
		uptime := time.Since(startTime).Round(time.Second).String()
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": serviceName,
			"uptime":  uptime,
		})
	}
}
