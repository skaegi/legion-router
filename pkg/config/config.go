package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the main configuration structure
type Config struct {
	Version string `yaml:"version" json:"version"`
	Rules   []Rule `yaml:"rules" json:"rules"`
}

// Rule represents a single filtering rule
type Rule struct {
	Name   string `yaml:"name" json:"name"`
	Action Action `yaml:"action" json:"action"`
	Order  int    `yaml:"order" json:"order"`
	Egress Egress `yaml:"egress,omitempty" json:"egress,omitempty"`
}

// Action represents allow or deny
type Action string

const (
	ActionAllow Action = "allow"
	ActionDeny  Action = "deny"
)

// Egress represents egress filtering criteria
type Egress struct {
	Protocols []Protocol `yaml:"protocols,omitempty" json:"protocols,omitempty"`
	Domains   []string   `yaml:"domains,omitempty" json:"domains,omitempty"`
	IPs       []string   `yaml:"ips,omitempty" json:"ips,omitempty"`
	Ports     []string   `yaml:"ports,omitempty" json:"ports,omitempty"`
}

// Protocol represents network protocols
type Protocol string

const (
	ProtocolTCP  Protocol = "tcp"
	ProtocolUDP  Protocol = "udp"
	ProtocolICMP Protocol = "icmp"
)

// Load reads and parses the configuration file
// Supports both YAML (.yaml, .yml) and JSON (.json) formats
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config

	// Detect format based on file extension
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	default:
		// Try YAML first (backward compatibility), then JSON
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			if jsonErr := json.Unmarshal(data, &cfg); jsonErr != nil {
				return nil, fmt.Errorf("failed to parse config as YAML or JSON: %w", err)
			}
		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Sort rules by order (lower number = higher priority)
	sort.Slice(cfg.Rules, func(i, j int) bool {
		return cfg.Rules[i].Order < cfg.Rules[j].Order
	})

	return &cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Version == "" {
		return fmt.Errorf("version is required")
	}

	if len(c.Rules) == 0 {
		return fmt.Errorf("at least one rule is required")
	}

	for i, rule := range c.Rules {
		if err := rule.Validate(); err != nil {
			return fmt.Errorf("rule %d (%s): %w", i, rule.Name, err)
		}
	}

	return nil
}

// Validate checks if a rule is valid
func (r *Rule) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("name is required")
	}

	if r.Action != ActionAllow && r.Action != ActionDeny {
		return fmt.Errorf("action must be 'allow' or 'deny'")
	}

	// Validate protocols
	for _, proto := range r.Egress.Protocols {
		if proto != ProtocolTCP && proto != ProtocolUDP && proto != ProtocolICMP {
			return fmt.Errorf("invalid protocol: %s", proto)
		}
	}

	// TODO: Add validation for IPs (CIDR notation), ports (ranges), etc.

	return nil
}
