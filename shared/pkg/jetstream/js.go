// Package jetstream provides a JetStream client for TenzoShare services.
// It handles stream creation (idempotent), publishing, and durable push consumers
// used by notification and audit services.
package jetstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// StreamDef describes a JetStream stream to be created if it does not exist.
type StreamDef struct {
	// Name is the stream name (e.g. "AUDIT").
	Name string
	// Subjects is the list of subject patterns the stream captures (e.g. ["AUDIT.*"]).
	Subjects []string
	// MaxAge is the maximum age for retained messages (0 = unlimited).
	MaxAge time.Duration
}

// Client wraps a NATS connection and its JetStream context.
type Client struct {
	nc *nats.Conn
	js jetstream.JetStream
}

// New connects to NATS and returns a ready Client.
func New(natsURL string) (*Client, error) {
	nc, err := nats.Connect(natsURL,
		nats.MaxReconnects(10),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("jetstream: connect to %s: %w", natsURL, err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("jetstream: create js context: %w", err)
	}
	return &Client{nc: nc, js: js}, nil
}

// EnsureStream creates a stream idempotently. If the stream already exists with
// the same config it is a no-op; if it exists with different subjects the
// subjects list is updated.
func (c *Client) EnsureStream(ctx context.Context, def StreamDef) error {
	cfg := jetstream.StreamConfig{
		Name:     def.Name,
		Subjects: def.Subjects,
		Storage:  jetstream.FileStorage,
		MaxAge:   def.MaxAge,
	}
	_, err := c.js.CreateOrUpdateStream(ctx, cfg)
	if err != nil {
		return fmt.Errorf("jetstream: ensure stream %q: %w", def.Name, err)
	}
	return nil
}

// Publish serialises msg as JSON and publishes it to subject.
func (c *Client) Publish(ctx context.Context, subject string, msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("jetstream: marshal message: %w", err)
	}
	_, err = c.js.Publish(ctx, subject, data)
	if err != nil {
		return fmt.Errorf("jetstream: publish to %q: %w", subject, err)
	}
	return nil
}

// Subscribe creates a durable push consumer and calls handler for every message.
// handler must return nil to acknowledge or a non-nil error to NAK (the message
// will be redelivered). This call blocks until ctx is done.
func (c *Client) Subscribe(
	ctx context.Context,
	streamName, consumerName, filterSubject string,
	handler func(subject string, data []byte) error,
) error {
	cons, err := c.js.CreateOrUpdateConsumer(ctx, streamName, jetstream.ConsumerConfig{
		Durable:        consumerName,
		FilterSubject:  filterSubject,
		AckPolicy:      jetstream.AckExplicitPolicy,
		DeliverPolicy:  jetstream.DeliverAllPolicy,
		MaxDeliver:     5,
		AckWait:        30 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("jetstream: create consumer %q on %q: %w", consumerName, streamName, err)
	}

	msgCtx, err := cons.Messages()
	if err != nil {
		return fmt.Errorf("jetstream: start message iter: %w", err)
	}
	defer msgCtx.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		msg, err := msgCtx.Next()
		if err != nil {
			if errors.Is(err, jetstream.ErrMsgIteratorClosed) {
				return nil
			}
			// transient error — keep going
			continue
		}
		if handlerErr := handler(msg.Subject(), msg.Data()); handlerErr != nil {
			_ = msg.Nak()
		} else {
			_ = msg.Ack()
		}
	}
}

// Close drains and closes the underlying NATS connection.
func (c *Client) Close() {
	_ = c.nc.Drain()
}

// SubscribeLast is like Subscribe but uses DeliverLastPolicy so the handler is
// called immediately with the most-recently-stored message on the subject (if
// any). Subsequent messages are delivered as they arrive. Ideal for config
// subjects where only the latest value matters.
func (c *Client) SubscribeLast(
	ctx context.Context,
	streamName, consumerName, filterSubject string,
	handler func(subject string, data []byte) error,
) error {
	cons, err := c.js.CreateOrUpdateConsumer(ctx, streamName, jetstream.ConsumerConfig{
		Durable:       consumerName,
		FilterSubject: filterSubject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverLastPolicy,
		MaxDeliver:    5,
		AckWait:       30 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("jetstream: create consumer %q on %q: %w", consumerName, streamName, err)
	}

	msgCtx, err := cons.Messages()
	if err != nil {
		return fmt.Errorf("jetstream: start message iter: %w", err)
	}
	defer msgCtx.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		msg, err := msgCtx.Next()
		if err != nil {
			if errors.Is(err, jetstream.ErrMsgIteratorClosed) {
				return nil
			}
			continue
		}
		if handlerErr := handler(msg.Subject(), msg.Data()); handlerErr != nil {
			_ = msg.Nak()
		} else {
			_ = msg.Ack()
		}
	}
}

// TenzoShare standard streams — call EnsureStreams at service startup for any
// service that produces or consumes events.
var TenzoStreams = []StreamDef{
	{
		Name:     "AUDIT",
		Subjects: []string{"AUDIT.*"},
		MaxAge:   90 * 24 * time.Hour, // 90 days
	},
	{
		Name:     "NOTIFICATIONS",
		Subjects: []string{"NOTIFICATIONS.*"},
		MaxAge:   7 * 24 * time.Hour, // 7 days
	},
	{
		Name:     "UPLOADS",
		Subjects: []string{"UPLOADS.*"},
		MaxAge:   7 * 24 * time.Hour,
	},
	{
		Name:     "CONFIG",
		Subjects: []string{"CONFIG.*"},
		MaxAge:   0, // keep forever — config messages are small and must survive restarts
	},
}

// EnsureStreams creates all TenzoShare standard JetStream streams idempotently.
func (c *Client) EnsureStreams(ctx context.Context) error {
	for _, def := range TenzoStreams {
		if err := c.EnsureStream(ctx, def); err != nil {
			return err
		}
	}
	return nil
}
