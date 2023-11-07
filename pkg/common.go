package pirelay

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type Config struct {
	ListenAddr string     `json:"listen_addr"`
	Location   [2]float64 `json:"location"`
	Timezone   string     `json:"timezone"`
	Relays     []*Relay   `json:"relays"`
	mu         *sync.Mutex
}

func New(timezone, listenAddr string, location [2]float64) *Config {
	return &Config{
		ListenAddr: listenAddr,
		Timezone:   timezone,
		Location:   location,
		mu:         &sync.Mutex{},
		Relays:     []*Relay{},
	}
}

var stdout, stderr = json.NewEncoder(os.Stdout), json.NewEncoder(os.Stderr)

func Log(isErr bool, format string, a ...any) error {
	if isErr {
		return stderr.Encode(map[string]interface{}{
			"error":     fmt.Sprintf(format, a...),
			"timestamp": time.Now(),
		})

	}
	return stdout.Encode(map[string]interface{}{
		"message":   fmt.Sprintf(format, a...),
		"timestamp": time.Now(),
	})
}
