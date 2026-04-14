package mqtt

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	server "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"

	"github.com/ruaan-deysel/unraid-management-agent/daemon/dto"
	"github.com/ruaan-deysel/unraid-management-agent/daemon/logger"
)

// EmbeddedBroker wraps mochi-mqtt for in-process MQTT brokering.
type EmbeddedBroker struct {
	srv       *server.Server
	port      int
	bindAll   bool
	running   atomic.Bool
	startedAt time.Time
	mu        sync.RWMutex
}

// NewEmbeddedBroker creates an EmbeddedBroker (not started yet).
// Returns an error if port is out of the valid range 1–65535.
func NewEmbeddedBroker(port int, bindAll bool) (*EmbeddedBroker, error) {
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("invalid embedded broker port %d: must be 1–65535", port)
	}
	return &EmbeddedBroker{port: port, bindAll: bindAll}, nil
}

// Start starts the embedded broker and blocks until the TCP listener is ready.
// If password is non-empty, requires username "unraid" and that password.
func (b *EmbeddedBroker) Start(_ context.Context, password string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.srv = server.New(nil)

	// Auth hook
	if password != "" {
		if err := b.srv.AddHook(new(auth.Hook), &auth.Options{
			Ledger: &auth.Ledger{
				Auth: auth.AuthRules{
					{Username: auth.RString("unraid"), Password: auth.RString(password), Allow: true},
				},
			},
		}); err != nil {
			return fmt.Errorf("adding auth hook: %w", err)
		}
	} else {
		if err := b.srv.AddHook(new(auth.AllowHook), nil); err != nil {
			return fmt.Errorf("adding allow hook: %w", err)
		}
	}

	// Determine bind address
	bindAddr := fmt.Sprintf("127.0.0.1:%d", b.port)
	if b.bindAll {
		bindAddr = fmt.Sprintf("0.0.0.0:%d", b.port)
		if password == "" {
			logger.Warning("Embedded MQTT broker: bind_all is true but no password set — broker is open to the network")
		}
	}

	tcp := listeners.NewTCP(listeners.Config{
		ID:      "embedded",
		Address: bindAddr,
	})
	if err := b.srv.AddListener(tcp); err != nil {
		return fmt.Errorf("adding TCP listener: %w", err)
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.LogPanicWithStack("Embedded MQTT broker", r)
			}
		}()
		if err := b.srv.Serve(); err != nil {
			logger.Error("Embedded MQTT broker stopped: %v", err)
		}
	}()

	// Poll until port is dialable (max 5 s)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", b.port), 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			b.running.Store(true)
			b.startedAt = time.Now()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("embedded broker did not start within 5 seconds on port %d", b.port)
}

// Stop gracefully stops the broker.
func (b *EmbeddedBroker) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.srv != nil {
		_ = b.srv.Close()
		b.running.Store(false)
	}
}

// Address returns the paho-compatible broker URL (always loopback).
func (b *EmbeddedBroker) Address() string {
	return fmt.Sprintf("tcp://127.0.0.1:%d", b.port)
}

// IsRunning returns true if the broker is running.
func (b *EmbeddedBroker) IsRunning() bool {
	return b.running.Load()
}

// GetStatus returns current broker status.
func (b *EmbeddedBroker) GetStatus() *dto.EmbeddedBrokerStatus {
	b.mu.RLock()
	defer b.mu.RUnlock()

	host := "127.0.0.1"
	if b.bindAll {
		host = "0.0.0.0"
	}

	status := &dto.EmbeddedBrokerStatus{
		Enabled: true,
		Running: b.running.Load(),
		Address: fmt.Sprintf("%s:%d", host, b.port),
	}

	if status.Running && !b.startedAt.IsZero() {
		status.UptimeSeconds = int64(time.Since(b.startedAt).Seconds())
		t := b.startedAt
		status.StartedAt = &t
	}

	if b.srv != nil {
		status.ClientCount = int(atomic.LoadInt64(&b.srv.Info.ClientsConnected))
	}

	return status
}
