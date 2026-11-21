package auth

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

type Limits struct {
	LoginPerPair       int
	LoginPerIP         int
	RegistrationsPerIP int
}

type Config struct {
	DatabaseURL string
	WSURL       string
	Limits      Limits
}

func LoadConfig(getenv func(string) string, listenAddress string) (Config, error) {
	result := Config{
		DatabaseURL: getenv("NEWMMO_DATABASE_URL"),
		WSURL:       getenv("NEWMMO_PUBLIC_WS_URL"),
		Limits:      Limits{LoginPerPair: 5, LoginPerIP: 20, RegistrationsPerIP: 5},
	}
	if result.DatabaseURL == "" {
		return Config{}, errors.New("NEWMMO_DATABASE_URL is required")
	}
	if result.WSURL == "" {
		_, port, err := net.SplitHostPort(listenAddress)
		if err != nil {
			return Config{}, errors.New("cannot derive dev WebSocket URL from listen address")
		}
		result.WSURL = "ws://" + net.JoinHostPort("127.0.0.1", port) + "/v1/ws"
	}
	ws, err := url.Parse(result.WSURL)
	if err != nil || (ws.Scheme != "ws" && ws.Scheme != "wss") || ws.Hostname() == "" ||
		ws.User != nil || ws.Path != "/v1/ws" || ws.RawQuery != "" || ws.ForceQuery || ws.Fragment != "" {
		return Config{}, errors.New("NEWMMO_PUBLIC_WS_URL must be a ws or wss URL ending in /v1/ws")
	}
	if ws.Scheme == "ws" && !strings.EqualFold(ws.Hostname(), "localhost") && !net.ParseIP(ws.Hostname()).IsLoopback() {
		return Config{}, errors.New("NEWMMO_PUBLIC_WS_URL requires wss outside loopback")
	}
	for _, setting := range []struct {
		name  string
		field *int
	}{
		{"NEWMMO_LOGIN_LIMIT_PAIR", &result.Limits.LoginPerPair},
		{"NEWMMO_LOGIN_LIMIT_IP", &result.Limits.LoginPerIP},
		{"NEWMMO_REGISTRATION_LIMIT_IP", &result.Limits.RegistrationsPerIP},
	} {
		if value := getenv(setting.name); value != "" {
			count, err := strconv.ParseInt(value, 10, 32)
			if err != nil || count <= 0 {
				return Config{}, fmt.Errorf("%s must be a positive integer", setting.name)
			}
			*setting.field = int(count)
		}
	}
	return result, nil
}
