# Embedded MQTT Broker Feature

**Date:** 2026-04-12  
**Status:** Planning  
**Author:** Scott Young

---

## Problem

Users who want MQTT/Home Assistant integration must install and operate a separate MQTT broker (Mosquitto, EMQX, etc.) before the agent can publish data. This is an unnecessary hurdle — especially for homelab users who just want plug-and-play HA discovery without standing up another container.

## Goal

Embed a lightweight, optional MQTT broker directly in the agent binary. When enabled, it starts on `127.0.0.1:1883` (or a user-configured port), and the agent's paho client connects to it automatically. External brokers remain fully supported; the embedded broker is additive.

---

## Library Decision: mochi-mqtt/server v2

**Import:** `github.com/mochi-mqtt/server/v2`  
**Version:** v2.7.9 (latest stable, MIT license)

**Why:**
- Pure Go, single binary — zero external dependencies
- Full MQTT v3.1.1 + v5 with retained messages and QoS 0/1/2 (both required for HA discovery)
- Hook-based auth (username/password optional ledger)
- In-memory by default — no disk writes, NAS-friendly RAM footprint
- Actively maintained; passes paho interop tests
- paho client connects to it identically to any external broker

---

## Existing MQTT Code Map

| File | Role | Touch for this feature? |
|------|------|------------------------|
| `daemon/domain/config.go` | `MQTTConfig` struct + `DefaultMQTTConfig()` + `ToDTOConfig()` | YES — add embedded broker fields |
| `daemon/domain/fileconfig.go` | `FileConfigMQTT` YAML struct | YES — add embedded broker YAML keys |
| `daemon/domain/context.go` | `MQTTConfig` attached to `domain.Context` | No change needed |
| `daemon/dto/mqtt.go` | `MQTTConfig` DTO, `MQTTStatus` DTO | YES — add embedded broker fields + status DTO |
| `daemon/services/mqtt/client.go` | paho wrapper: `Connect()`, `Disconnect()`, `Publish*()` | No change — broker sits behind it transparently |
| `daemon/services/mqtt/discovery.go` | HA entity discovery publishing | No change |
| `daemon/services/mqtt/commands.go` | HA command topic subscriptions | No change |
| `daemon/services/orchestrator.go:initializeMQTT()` | Creates paho client, connects, starts event subscriber | YES — start embedded broker before paho connects |
| `daemon/services/api/handlers.go` | MQTT REST handlers | YES — add broker status endpoint |
| `daemon/services/api/server.go` | Route registration | YES — register new route |
| `daemon/services/mcp/server.go` | MCP tools | YES — expose broker status in `get_mqtt_status` |

---

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                   Orchestrator                       │
│                                                      │
│  initializeMQTT()                                   │
│    1. if embedded_broker.enabled:                   │
│         EmbeddedBroker.Start()  ← NEW               │
│         wait for listener ready                     │
│    2. point paho client at localhost:<port>         │
│    3. paho client.Connect()                         │
│    4. start subscribeMQTTEvents()                   │
│                                                      │
│  shutdown order:                                     │
│    1. paho client.Disconnect()                      │
│    2. EmbeddedBroker.Stop()     ← NEW               │
└─────────────────────────────────────────────────────┘

External MQTT client (HA, Node-RED, etc.)
        │
        │ TCP:1883  (if embedded_broker.bind_all = true)
        ▼
┌─────────────────┐     ┌──────────────────────────┐
│ EmbeddedBroker  │────▶│ paho.mqtt.Client         │
│ mochi-mqtt v2   │     │ (existing client.go)     │
│ 127.0.0.1:1883  │     │ → Publish* methods       │
└─────────────────┘     │ → HA discovery           │
                        │ → command subscriptions  │
                        └──────────────────────────┘
```

When `embedded_broker.bind_all = true`, external clients (e.g. Home Assistant running elsewhere) can also connect to the broker on the host's LAN IP. Otherwise it binds to loopback only (agent use only).

---

## Implementation Plan

### Phase 1 — Dependency

```bash
go get github.com/mochi-mqtt/server/v2@v2.7.9
```

### Phase 2 — Embedded Broker Package

**New file: `daemon/services/mqtt/broker.go`**

```go
package mqtt

import (
    "context"
    "fmt"
    "net"

    server "github.com/mochi-mqtt/server/v2"
    "github.com/mochi-mqtt/server/v2/hooks/auth"
    "github.com/mochi-mqtt/server/v2/listeners"
    "github.com/ruaan-deysel/unraid-management-agent/daemon/logger"
)

// EmbeddedBroker wraps a mochi-mqtt server for in-process MQTT brokering.
type EmbeddedBroker struct {
    server  *server.Server
    port    int
    bindAll bool
    ready   chan struct{}
}

// NewEmbeddedBroker creates an embedded MQTT broker.
// port: TCP port to listen on (default 1883)
// bindAll: if true, bind to 0.0.0.0 (LAN-accessible); otherwise 127.0.0.1 only
// password: if non-empty, require username "unraid" + this password
func NewEmbeddedBroker(port int, bindAll bool, password string) *EmbeddedBroker {
    return &EmbeddedBroker{port: port, bindAll: bindAll, ready: make(chan struct{})}
}

// Start starts the embedded broker and blocks until the listener is ready.
func (b *EmbeddedBroker) Start(ctx context.Context, password string) error { ... }

// Stop gracefully stops the broker.
func (b *EmbeddedBroker) Stop() { ... }

// Address returns the TCP address the paho client should connect to.
func (b *EmbeddedBroker) Address() string {
    return fmt.Sprintf("tcp://127.0.0.1:%d", b.port)
}

// ClientCount returns the number of currently connected clients.
func (b *EmbeddedBroker) ClientCount() int { ... }
```

Key implementation notes:
- Use `auth.AllowHook` when no password set; `auth.Hook` with a `auth.Ledger` when password is configured
- Signal `ready` from a custom hook's `OnSysInfoTick` or by checking the listener's `net.Listener`
- Wrap `server.Serve()` in a goroutine; `ctx.Done()` cancels via `server.Close()`

### Phase 3 — Config Changes

**`daemon/domain/config.go`** — add to `MQTTConfig`:
```go
// Embedded broker settings
EmbeddedBroker         bool   `json:"embedded_broker"`
EmbeddedBrokerPort     int    `json:"embedded_broker_port"`
EmbeddedBrokerBindAll  bool   `json:"embedded_broker_bind_all"`
EmbeddedBrokerPassword string `json:"-"`
```

Update `DefaultMQTTConfig()`:
```go
EmbeddedBrokerPort: 1883,
```

Update `ToDTOConfig()` — when embedded broker is enabled, override `Broker` to point at localhost.

**`daemon/domain/fileconfig.go`** — add to `FileConfigMQTT`:
```yaml
embedded_broker:          true
embedded_broker_port:     1883
embedded_broker_bind_all: false    # set true to expose broker on LAN
embedded_broker_password: ""       # empty = no auth
```

### Phase 4 — DTO Changes

**`daemon/dto/mqtt.go`** — add to `MQTTConfig` DTO:
```go
EmbeddedBroker         bool `json:"embedded_broker" example:"false"`
EmbeddedBrokerPort     int  `json:"embedded_broker_port" example:"1883"`
EmbeddedBrokerBindAll  bool `json:"embedded_broker_bind_all" example:"false"`
```

Add new DTO:
```go
// EmbeddedBrokerStatus reports the state of the built-in MQTT broker.
type EmbeddedBrokerStatus struct {
    Enabled     bool      `json:"enabled"`
    Running     bool      `json:"running"`
    Address     string    `json:"address"`       // "127.0.0.1:1883" or "0.0.0.0:1883"
    ClientCount int       `json:"client_count"`
    Uptime      int64     `json:"uptime_seconds"`
    StartedAt   *time.Time `json:"started_at,omitempty"`
}
```

Add `EmbeddedBroker *EmbeddedBrokerStatus` to `MQTTStatus`.

### Phase 5 — Orchestrator Integration

**`daemon/services/orchestrator.go`** — add `embeddedBroker *mqtt.EmbeddedBroker` to `Orchestrator` struct.

Update `initializeMQTT()`:
```go
// Start embedded broker first if configured
if o.ctx.MQTTConfig.EmbeddedBroker {
    b := mqtt.NewEmbeddedBroker(
        o.ctx.MQTTConfig.EmbeddedBrokerPort,
        o.ctx.MQTTConfig.EmbeddedBrokerBindAll,
        o.ctx.MQTTConfig.EmbeddedBrokerPassword,
    )
    if err := b.Start(ctx, o.ctx.MQTTConfig.EmbeddedBrokerPassword); err != nil {
        logger.Error("Failed to start embedded MQTT broker: %v", err)
        return
    }
    o.embeddedBroker = b
    logger.Success("Embedded MQTT broker started on %s", b.Address())
    // Override broker URL so paho connects to embedded broker
    mqttConfig.Broker = b.Address()
}
```

Update shutdown (step 4, before step 5):
```go
if o.embeddedBroker != nil {
    o.embeddedBroker.Stop()
    logger.Info("Embedded MQTT broker stopped")
}
```

### Phase 6 — REST API

**New handler in `daemon/services/api/handlers.go`:**
```
GET /api/mqtt/broker  → embedded broker status
```

Response: `EmbeddedBrokerStatus` JSON.

**Route registration in `daemon/services/api/server.go`:**
```go
r.HandleFunc("/api/mqtt/broker", s.handleMQTTBrokerStatus).Methods("GET")
```

No start/stop endpoints in v1 — config file controls whether it runs.

### Phase 7 — MCP Exposure

**`daemon/services/mcp/server.go`** — add `embedded_broker` field to the existing `get_mqtt_status` tool response. No new tool needed.

### Phase 8 — Config File Docs

Update `docs/integrations/mqtt.md` with embedded broker section:
- How to enable (YAML keys)
- When to use vs. external broker
- Exposing to LAN (bind_all)
- Password protection

---

## Config File Example

```yaml
mqtt:
  enabled: true
  # Use the embedded broker (no external Mosquitto needed)
  embedded_broker: true
  embedded_broker_port: 1883
  embedded_broker_bind_all: false   # true = HA running on different host can connect
  embedded_broker_password: ""      # empty = allow all connections

  # broker, username, password are ignored when embedded_broker: true
  topic_prefix: unraid
  home_assistant: true
  ha_prefix: homeassistant
```

---

## Testing Plan

1. Unit test `EmbeddedBroker.Start()` / `Stop()` lifecycle
2. Integration test: start broker, connect paho client, publish + receive a retained message
3. Auth test: broker with password rejects unauthorized clients
4. Orchestrator test: `initializeMQTT()` with `EmbeddedBroker: true` wires paho to localhost
5. End-to-end on Unraid hardware: HA MQTT integration with embedded broker, confirm discovery entities appear

---

## Non-Goals (v1)

- Persistence across restarts (in-memory only; retained messages reset on agent restart)
- Clustering / bridging to external brokers
- TLS on the embedded broker listener (loopback is safe; LAN users can use reverse proxy)
- Web UI for broker management

---

## Open Questions

1. **Port conflict**: what happens if 1883 is already taken by Mosquitto? → Error at startup with clear message; user should set a different `embedded_broker_port` or disable embedded broker.
2. **bind_all auth**: when `bind_all: true`, should we require a password? → Warn in logs if `bind_all: true` and no password set; do not block startup.
3. **Retained message persistence**: should we offer optional Bolt/Badger persistence so HA doesn't lose entity state on agent restart? → Deferred to v2 of this feature.
