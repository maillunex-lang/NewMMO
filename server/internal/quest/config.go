package quest

import (
	"encoding/json"
	"os"
	"strings"
	"time"
)

// RegionConfig gates AVAILABLE by open_at (docs/PACING.md §3, quest-engine §1).
type RegionConfig struct {
	OpenAt               map[string]time.Time // REG-0N → UTC open time; missing = open
	EarlyAccessAccounts  map[string]struct{}
	InteractablesPath    string
}

func LoadRegionConfig(getenv func(string) string) RegionConfig {
	cfg := RegionConfig{
		OpenAt:              map[string]time.Time{},
		EarlyAccessAccounts: map[string]struct{}{},
		InteractablesPath:   getenv("NEWMMO_INTERACTABLES_PATH"),
	}
	if cfg.InteractablesPath == "" {
		cfg.InteractablesPath = "content/interactables/reg-01.json"
	}
	raw := strings.TrimSpace(getenv("NEWMMO_REGIONS_JSON"))
	if raw == "" {
		return cfg
	}
	var parsed struct {
		Regions map[string]struct {
			OpenAt string `json:"open_at"`
		} `json:"regions"`
		EarlyAccess []string `json:"early_access_accounts"`
	}
	if json.Unmarshal([]byte(raw), &parsed) != nil {
		return cfg
	}
	for id, r := range parsed.Regions {
		if r.OpenAt == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339, r.OpenAt); err == nil {
			cfg.OpenAt[id] = t.UTC()
		}
	}
	for _, id := range parsed.EarlyAccess {
		cfg.EarlyAccessAccounts[id] = struct{}{}
	}
	return cfg
}

func (c RegionConfig) RegionOpen(regionID, accountID string, now time.Time) bool {
	if _, ok := c.EarlyAccessAccounts[accountID]; ok {
		return true
	}
	openAt, set := c.OpenAt[regionID]
	if !set || openAt.IsZero() {
		return true
	}
	return !now.Before(openAt)
}

// ResolveInteractablesPath tries env, then walks up from cwd for the repo file.
func ResolveInteractablesPath(preferred string) string {
	if preferred != "" {
		if _, err := os.Stat(preferred); err == nil {
			return preferred
		}
	}
	candidates := []string{
		"content/interactables/reg-01.json",
		"../content/interactables/reg-01.json",
		"../../content/interactables/reg-01.json",
		"../../../content/interactables/reg-01.json",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return preferred
}
