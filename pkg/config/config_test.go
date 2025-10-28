package config

import (
	"os"
	"testing"
)

func TestLoadYAML(t *testing.T) {
	// Create a temporary YAML config file
	yamlContent := `version: "1.0"
rules:
  - name: allow-dns
    action: allow
    order: 100
    egress:
      protocols: [udp, tcp]
      ips: ["8.8.8.8"]
      ports: ["53"]
`
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(yamlContent)); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	// Test loading
	cfg, err := Load(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to load YAML config: %v", err)
	}

	if cfg.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", cfg.Version)
	}

	if len(cfg.Rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(cfg.Rules))
	}

	if cfg.Rules[0].Name != "allow-dns" {
		t.Errorf("Expected rule name 'allow-dns', got '%s'", cfg.Rules[0].Name)
	}

	if cfg.Rules[0].Action != ActionAllow {
		t.Errorf("Expected action 'allow', got '%s'", cfg.Rules[0].Action)
	}
}

func TestLoadJSON(t *testing.T) {
	// Create a temporary JSON config file
	jsonContent := `{
  "version": "1.0",
  "rules": [
    {
      "name": "allow-dns",
      "action": "allow",
      "order": 100,
      "egress": {
        "protocols": ["udp", "tcp"],
        "ips": ["8.8.8.8"],
        "ports": ["53"]
      }
    }
  ]
}`
	tmpfile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(jsonContent)); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	// Test loading
	cfg, err := Load(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to load JSON config: %v", err)
	}

	if cfg.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", cfg.Version)
	}

	if len(cfg.Rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(cfg.Rules))
	}

	if cfg.Rules[0].Name != "allow-dns" {
		t.Errorf("Expected rule name 'allow-dns', got '%s'", cfg.Rules[0].Name)
	}

	if cfg.Rules[0].Action != ActionAllow {
		t.Errorf("Expected action 'allow', got '%s'", cfg.Rules[0].Action)
	}

	if len(cfg.Rules[0].Egress.Protocols) != 2 {
		t.Errorf("Expected 2 protocols, got %d", len(cfg.Rules[0].Egress.Protocols))
	}

	if cfg.Rules[0].Egress.Protocols[0] != ProtocolUDP {
		t.Errorf("Expected protocol 'udp', got '%s'", cfg.Rules[0].Egress.Protocols[0])
	}
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				Version: "1.0",
				Rules: []Rule{
					{
						Name:   "test-rule",
						Action: ActionAllow,
						Order:  100,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing version",
			cfg: Config{
				Rules: []Rule{
					{
						Name:   "test-rule",
						Action: ActionAllow,
						Order:  100,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "no rules",
			cfg: Config{
				Version: "1.0",
				Rules:   []Rule{},
			},
			wantErr: true,
		},
		{
			name: "invalid action",
			cfg: Config{
				Version: "1.0",
				Rules: []Rule{
					{
						Name:   "test-rule",
						Action: "invalid",
						Order:  100,
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
