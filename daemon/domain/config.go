// Package domain provides core domain models and configuration structures for the Unraid Management Agent.
package domain

import (
	"fmt"

	"github.com/ruaan-deysel/unraid-management-agent/daemon/dto"
)

// MQTTConfig holds MQTT broker connection and publishing settings.
type MQTTConfig struct {
	// Connection settings
	Enabled  bool   `json:"enabled"`
	Broker   string `json:"broker"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"-"` // Never serialize password
	ClientID string `json:"client_id"`

	// TLS settings
	UseTLS             bool `json:"use_tls"`
	InsecureSkipVerify bool `json:"insecure_skip_verify"`

	// Publishing settings
	TopicPrefix    string `json:"topic_prefix"`
	QoS            int    `json:"qos"`
	RetainMessages bool   `json:"retain_messages"`

	// Home Assistant integration
	HomeAssistantMode   bool   `json:"home_assistant_mode"`
	HomeAssistantPrefix string `json:"home_assistant_prefix"`
	DiscoveryEnabled    bool   `json:"discovery_enabled"`

	// Embedded broker settings
	EmbeddedBrokerEnabled  bool   `json:"embedded_broker_enabled"`
	EmbeddedBrokerPort     int    `json:"embedded_broker_port"`
	EmbeddedBrokerBindAll  bool   `json:"embedded_broker_bind_all"`
	EmbeddedBrokerPassword string `json:"-"`
}

// Config holds the application configuration settings.
type Config struct {
	Version    string `json:"version"`
	Port       int    `json:"port"`
	CORSOrigin string `json:"cors_origin,omitempty"`
}

// DefaultMQTTConfig returns the default MQTT configuration.
func DefaultMQTTConfig() MQTTConfig {
	return MQTTConfig{
		Enabled:             false,
		Broker:              "",
		Port:                1883,
		Username:            "",
		Password:            "",
		ClientID:            "unraid-management-agent",
		UseTLS:              false,
		InsecureSkipVerify:  false,
		TopicPrefix:         "unraid",
		QoS:                 0,
		RetainMessages:      true,
		HomeAssistantMode:   false,
		HomeAssistantPrefix: "homeassistant",
		DiscoveryEnabled:    true,
		EmbeddedBrokerPort:  1883,
	}
}

// ToDTOConfig converts domain.MQTTConfig to dto.MQTTConfig for use with the MQTT client.
func (c *MQTTConfig) ToDTOConfig() *dto.MQTTConfig {
	// Build broker URL with protocol and port
	broker := c.Broker
	if c.EmbeddedBrokerEnabled {
		// Paho connects to the embedded broker on loopback
		broker = fmt.Sprintf("tcp://127.0.0.1:%d", c.EmbeddedBrokerPort)
	} else if broker != "" && c.Port > 0 {
		protocol := "tcp"
		if c.UseTLS {
			protocol = "ssl"
		}
		broker = fmt.Sprintf("%s://%s:%d", protocol, c.Broker, c.Port)
	}

	return &dto.MQTTConfig{
		Enabled:               c.Enabled || c.EmbeddedBrokerEnabled,
		Broker:                broker,
		ClientID:              c.ClientID,
		Username:              c.Username,
		Password:              c.Password,
		TopicPrefix:           c.TopicPrefix,
		QoS:                   c.QoS,
		RetainMessages:        c.RetainMessages,
		ConnectTimeout:        30,
		KeepAlive:             60,
		CleanSession:          true,
		AutoReconnect:         true,
		HomeAssistantMode:     c.HomeAssistantMode,
		HADiscoveryPrefix:     c.HomeAssistantPrefix,
		EmbeddedBrokerEnabled: c.EmbeddedBrokerEnabled,
		EmbeddedBrokerPort:    c.EmbeddedBrokerPort,
		EmbeddedBrokerBindAll: c.EmbeddedBrokerBindAll,
	}
}
