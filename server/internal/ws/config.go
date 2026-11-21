package ws

import (
	"fmt"
	"strconv"
	"time"
)

const (
	Subprotocol     = "newmmo.v1"
	inputLimit      = 16 << 10
	outputLimit     = 256 << 10
	ShutdownTimeout = 10 * time.Second
)

type Config struct {
	Handshakes, Pending, Rate, Burst, MutationRate, Queue, Violations int
	AuthTimeout, Heartbeat, PongTimeout, WriteTimeout                 time.Duration
}

func DefaultConfig() Config {
	return Config{
		Handshakes: 10, Pending: 3, Rate: 20, Burst: 40, MutationRate: 5,
		Queue: 256, Violations: 3, AuthTimeout: 5 * time.Second, Heartbeat: 25 * time.Second,
		PongTimeout: 15 * time.Second, WriteTimeout: 10 * time.Second,
	}
}

func LoadConfig(getenv func(string) string) (Config, error) {
	cfg := DefaultConfig()
	for _, field := range []struct {
		key     string
		value   *int
		minimum int
	}{
		{"NEWMMO_WS_HANDSHAKES_PER_MINUTE", &cfg.Handshakes, 1},
		{"NEWMMO_WS_PREAUTH_PER_IP", &cfg.Pending, 1},
		{"NEWMMO_WS_COMMANDS_PER_SECOND", &cfg.Rate, 1},
		{"NEWMMO_WS_COMMAND_BURST", &cfg.Burst, 1},
		{"NEWMMO_WS_MUTATIONS_PER_SECOND", &cfg.MutationRate, 1},
		{"NEWMMO_WS_QUEUE_SIZE", &cfg.Queue, 3},
		{"NEWMMO_WS_RATE_VIOLATIONS", &cfg.Violations, 1},
	} {
		if raw := getenv(field.key); raw != "" {
			n, err := strconv.ParseInt(raw, 10, 32)
			if err != nil || n < int64(field.minimum) {
				return Config{}, fmt.Errorf("%s must be an integer of at least %d", field.key, field.minimum)
			}
			*field.value = int(n)
		}
	}
	for _, field := range []struct {
		key   string
		value *time.Duration
	}{
		{"NEWMMO_WS_PREAUTH_TIMEOUT", &cfg.AuthTimeout},
		{"NEWMMO_WS_HEARTBEAT_INTERVAL", &cfg.Heartbeat},
		{"NEWMMO_WS_PONG_TIMEOUT", &cfg.PongTimeout},
		{"NEWMMO_WS_WRITE_TIMEOUT", &cfg.WriteTimeout},
	} {
		if raw := getenv(field.key); raw != "" {
			d, err := time.ParseDuration(raw)
			if err != nil || d < time.Millisecond {
				return Config{}, fmt.Errorf("%s must be a duration of at least 1ms", field.key)
			}
			*field.value = d
		}
	}
	return cfg, nil
}
