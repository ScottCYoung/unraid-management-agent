package mqtt

import (
	"net"
	"testing"
	"time"
)

// findFreePort returns a free TCP port on localhost.
func findFreePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not find free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

func TestNewEmbeddedBroker(t *testing.T) {
	b, err := NewEmbeddedBroker(1883, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.port != 1883 {
		t.Errorf("port = %d, want 1883", b.port)
	}
	if b.bindAll {
		t.Error("bindAll should be false")
	}
	if b.IsRunning() {
		t.Error("should not be running before Start()")
	}
	if got := b.Address(); got != "tcp://127.0.0.1:1883" {
		t.Errorf("Address() = %q, want %q", got, "tcp://127.0.0.1:1883")
	}
}

func TestNewEmbeddedBroker_InvalidPort(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{name: "zero", port: 0},
		{name: "negative", port: -1},
		{name: "too large", port: 65536},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewEmbeddedBroker(tt.port, false)
			if err == nil {
				t.Errorf("expected error for port %d", tt.port)
			}
		})
	}
}

func TestEmbeddedBroker_GetStatus_NotStarted(t *testing.T) {
	b, err := NewEmbeddedBroker(1883, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := b.GetStatus()
	if !s.Enabled {
		t.Error("Enabled should be true")
	}
	if s.Running {
		t.Error("Running should be false before Start()")
	}
	if s.Address != "127.0.0.1:1883" {
		t.Errorf("Address = %q, want %q", s.Address, "127.0.0.1:1883")
	}
	if s.ClientCount != 0 {
		t.Errorf("ClientCount = %d, want 0", s.ClientCount)
	}
}

func TestEmbeddedBroker_Stop_NotStarted(t *testing.T) {
	b, err := NewEmbeddedBroker(1883, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Must not panic
	b.Stop()
}

func TestEmbeddedBroker_StartStop(t *testing.T) {
	port := findFreePort(t)
	b, err := NewEmbeddedBroker(port, false)
	if err != nil {
		t.Fatalf("NewEmbeddedBroker: %v", err)
	}

	if err := b.Start(t.Context(), ""); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !b.IsRunning() {
		t.Error("IsRunning() should be true after Start()")
	}

	// Confirm port is dialable
	conn, err := net.DialTimeout("tcp", b.Address()[len("tcp://"):], time.Second)
	if err != nil {
		t.Fatalf("broker port not dialable after Start(): %v", err)
	}
	conn.Close()

	// Check status
	s := b.GetStatus()
	if !s.Running {
		t.Error("GetStatus().Running should be true")
	}
	if s.UptimeSeconds < 0 {
		t.Errorf("UptimeSeconds = %d, want >= 0", s.UptimeSeconds)
	}
	if s.StartedAt == nil {
		t.Error("StartedAt should not be nil when running")
	}

	b.Stop()
	if b.IsRunning() {
		t.Error("IsRunning() should be false after Stop()")
	}
}

func TestEmbeddedBroker_Start_PortConflict(t *testing.T) {
	// Hold a port open
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not bind port: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	b, err := NewEmbeddedBroker(port, false)
	if err != nil {
		t.Fatalf("NewEmbeddedBroker: %v", err)
	}
	err = b.Start(t.Context(), "")
	if err == nil {
		b.Stop()
		t.Fatal("expected error when port is already in use")
	}
}
