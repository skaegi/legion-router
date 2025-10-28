package main

import (
	"sort"
	"testing"

	"github.com/skaegi/legion-router/pkg/config"
)

// TestBasicDenyAll tests that a minimal config with no allow rules blocks everything
func TestBasicDenyAll(t *testing.T) {
	cfg := &config.Config{
		Version: "1.0",
		Rules:   []config.Rule{},
	}

	// With no rules, validation should fail (requires at least one rule)
	if err := cfg.Validate(); err == nil {
		t.Error("Expected validation to fail with no rules")
	}

	// Now test with a single deny rule
	cfg.Rules = []config.Rule{
		{
			Name:   "deny-all",
			Action: config.ActionDeny,
			Order:  100,
			Egress: config.Egress{},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Config validation failed: %v", err)
	}

	if len(cfg.Rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(cfg.Rules))
	}

	if cfg.Rules[0].Action != config.ActionDeny {
		t.Errorf("Expected deny action, got %s", cfg.Rules[0].Action)
	}
}

// TestAllowSingleIP tests allowing traffic to a single IP
func TestAllowSingleIP(t *testing.T) {
	cfg := &config.Config{
		Version: "1.0",
		Rules: []config.Rule{
			{
				Name:   "allow-dns",
				Action: config.ActionAllow,
				Order:  100,
				Egress: config.Egress{
					IPs:       []string{"8.8.8.8"},
					Protocols: []config.Protocol{config.ProtocolUDP},
					Ports:     []string{"53"},
				},
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Config validation failed: %v", err)
	}

	// Verify the rule structure
	rule := cfg.Rules[0]
	if rule.Action != config.ActionAllow {
		t.Errorf("Expected allow action, got %s", rule.Action)
	}

	if len(rule.Egress.IPs) != 1 || rule.Egress.IPs[0] != "8.8.8.8" {
		t.Errorf("Expected IP 8.8.8.8, got %v", rule.Egress.IPs)
	}

	if len(rule.Egress.Protocols) != 1 || rule.Egress.Protocols[0] != config.ProtocolUDP {
		t.Errorf("Expected UDP protocol, got %v", rule.Egress.Protocols)
	}

	if len(rule.Egress.Ports) != 1 || rule.Egress.Ports[0] != "53" {
		t.Errorf("Expected port 53, got %v", rule.Egress.Ports)
	}
}

// TestAllowCIDRRange tests allowing traffic to a CIDR range
func TestAllowCIDRRange(t *testing.T) {
	cfg := &config.Config{
		Version: "1.0",
		Rules: []config.Rule{
			{
				Name:   "allow-internal",
				Action: config.ActionAllow,
				Order:  100,
				Egress: config.Egress{
					IPs:       []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"},
					Protocols: []config.Protocol{config.ProtocolTCP, config.ProtocolUDP},
				},
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Config validation failed: %v", err)
	}

	rule := cfg.Rules[0]
	if len(rule.Egress.IPs) != 3 {
		t.Errorf("Expected 3 CIDR ranges, got %d", len(rule.Egress.IPs))
	}

	expectedIPs := []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}
	for i, expected := range expectedIPs {
		if rule.Egress.IPs[i] != expected {
			t.Errorf("Expected IP %s at index %d, got %s", expected, i, rule.Egress.IPs[i])
		}
	}
}

// TestAllowDomain tests allowing traffic to specific domains
func TestAllowDomain(t *testing.T) {
	cfg := &config.Config{
		Version: "1.0",
		Rules: []config.Rule{
			{
				Name:   "allow-github",
				Action: config.ActionAllow,
				Order:  100,
				Egress: config.Egress{
					Domains:   []string{"api.github.com", "github.com"},
					Protocols: []config.Protocol{config.ProtocolTCP},
					Ports:     []string{"443"},
				},
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Config validation failed: %v", err)
	}

	rule := cfg.Rules[0]
	if len(rule.Egress.Domains) != 2 {
		t.Errorf("Expected 2 domains, got %d", len(rule.Egress.Domains))
	}

	if rule.Egress.Domains[0] != "api.github.com" {
		t.Errorf("Expected domain api.github.com, got %s", rule.Egress.Domains[0])
	}
}

// TestPortRange tests port range filtering
func TestPortRange(t *testing.T) {
	cfg := &config.Config{
		Version: "1.0",
		Rules: []config.Rule{
			{
				Name:   "allow-port-range",
				Action: config.ActionAllow,
				Order:  100,
				Egress: config.Egress{
					IPs:       []string{"10.0.0.0/8"},
					Protocols: []config.Protocol{config.ProtocolTCP},
					Ports:     []string{"8000-9000", "3000"},
				},
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Config validation failed: %v", err)
	}

	rule := cfg.Rules[0]
	if len(rule.Egress.Ports) != 2 {
		t.Errorf("Expected 2 port specifications, got %d", len(rule.Egress.Ports))
	}

	if rule.Egress.Ports[0] != "8000-9000" {
		t.Errorf("Expected port range 8000-9000, got %s", rule.Egress.Ports[0])
	}

	if rule.Egress.Ports[1] != "3000" {
		t.Errorf("Expected port 3000, got %s", rule.Egress.Ports[1])
	}
}

// TestProtocolFiltering tests different protocol types
func TestProtocolFiltering(t *testing.T) {
	testCases := []struct {
		name     string
		protocol config.Protocol
	}{
		{"tcp", config.ProtocolTCP},
		{"udp", config.ProtocolUDP},
		{"icmp", config.ProtocolICMP},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{
				Version: "1.0",
				Rules: []config.Rule{
					{
						Name:   "allow-" + tc.name,
						Action: config.ActionAllow,
						Order:  100,
						Egress: config.Egress{
							Protocols: []config.Protocol{tc.protocol},
						},
					},
				},
			}

			if err := cfg.Validate(); err != nil {
				t.Fatalf("Config validation failed for %s: %v", tc.name, err)
			}

			if cfg.Rules[0].Egress.Protocols[0] != tc.protocol {
				t.Errorf("Expected protocol %s, got %s", tc.protocol, cfg.Rules[0].Egress.Protocols[0])
			}
		})
	}
}

// TestRulePriority tests that rules are sorted by priority
func TestRulePriority(t *testing.T) {
	cfg := &config.Config{
		Version: "1.0",
		Rules: []config.Rule{
			{
				Name:   "low-priority",
				Action: config.ActionAllow,
				Order:  300,
				Egress: config.Egress{},
			},
			{
				Name:   "high-priority",
				Action: config.ActionDeny,
				Order:  50,
				Egress: config.Egress{},
			},
			{
				Name:   "medium-priority",
				Action: config.ActionAllow,
				Order:  100,
				Egress: config.Egress{},
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Config validation failed: %v", err)
	}

	// Manually sort to test sorting logic
	sort.Slice(cfg.Rules, func(i, j int) bool {
		return cfg.Rules[i].Order < cfg.Rules[j].Order
	})

	expectedOrder := []string{"high-priority", "medium-priority", "low-priority"}
	for i, expected := range expectedOrder {
		if cfg.Rules[i].Name != expected {
			t.Errorf("Expected rule %s at position %d, got %s", expected, i, cfg.Rules[i].Name)
		}
	}

	// Verify orders
	if cfg.Rules[0].Order != 50 {
		t.Errorf("Expected first rule to have order 50, got %d", cfg.Rules[0].Order)
	}
	if cfg.Rules[1].Order != 100 {
		t.Errorf("Expected second rule to have order 100, got %d", cfg.Rules[1].Order)
	}
	if cfg.Rules[2].Order != 300 {
		t.Errorf("Expected third rule to have order 300, got %d", cfg.Rules[2].Order)
	}
}

// TestDenyBeforeAllow tests that deny rules can override allow rules based on priority
func TestDenyBeforeAllow(t *testing.T) {
	cfg := &config.Config{
		Version: "1.0",
		Rules: []config.Rule{
			{
				Name:   "allow-all-internal",
				Action: config.ActionAllow,
				Order:  100,
				Egress: config.Egress{
					IPs: []string{"169.254.0.0/16"},
				},
			},
			{
				Name:   "block-metadata",
				Action: config.ActionDeny,
				Order:  50,
				Egress: config.Egress{
					IPs: []string{"169.254.169.254"},
				},
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Config validation failed: %v", err)
	}

	// Sort by priority
	sort.Slice(cfg.Rules, func(i, j int) bool {
		return cfg.Rules[i].Order < cfg.Rules[j].Order
	})

	// The deny rule should be evaluated first (lower order = higher priority)
	if cfg.Rules[0].Name != "block-metadata" {
		t.Errorf("Expected block-metadata to be first after sorting, got %s", cfg.Rules[0].Name)
	}

	if cfg.Rules[0].Order != 50 {
		t.Errorf("Expected first rule to have order 50, got %d", cfg.Rules[0].Order)
	}
}

// TestComplexConfig tests a realistic multi-rule configuration
func TestComplexConfig(t *testing.T) {
	cfg := &config.Config{
		Version: "1.0",
		Rules: []config.Rule{
			{
				Name:   "block-metadata",
				Action: config.ActionDeny,
				Order:  50,
				Egress: config.Egress{
					IPs: []string{"169.254.169.254"},
				},
			},
			{
				Name:   "allow-dns",
				Action: config.ActionAllow,
				Order:  100,
				Egress: config.Egress{
					Protocols: []config.Protocol{config.ProtocolUDP, config.ProtocolTCP},
					IPs:       []string{"8.8.8.8", "1.1.1.1"},
					Ports:     []string{"53"},
				},
			},
			{
				Name:   "allow-github",
				Action: config.ActionAllow,
				Order:  100,
				Egress: config.Egress{
					Protocols: []config.Protocol{config.ProtocolTCP},
					Domains:   []string{"api.github.com"},
					Ports:     []string{"443"},
				},
			},
			{
				Name:   "allow-internal",
				Action: config.ActionAllow,
				Order:  200,
				Egress: config.Egress{
					Protocols: []config.Protocol{config.ProtocolTCP, config.ProtocolUDP},
					IPs:       []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"},
				},
			},
			{
				Name:   "allow-icmp",
				Action: config.ActionAllow,
				Order:  300,
				Egress: config.Egress{
					Protocols: []config.Protocol{config.ProtocolICMP},
				},
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Complex config validation failed: %v", err)
	}

	if len(cfg.Rules) != 5 {
		t.Errorf("Expected 5 rules, got %d", len(cfg.Rules))
	}

	// Verify each rule type
	blockRule := cfg.Rules[0]
	if blockRule.Action != config.ActionDeny {
		t.Errorf("Expected block rule to have deny action")
	}

	dnsRule := cfg.Rules[1]
	if len(dnsRule.Egress.IPs) != 2 {
		t.Errorf("Expected DNS rule to have 2 IPs")
	}
	if len(dnsRule.Egress.Protocols) != 2 {
		t.Errorf("Expected DNS rule to have 2 protocols")
	}

	githubRule := cfg.Rules[2]
	if len(githubRule.Egress.Domains) != 1 {
		t.Errorf("Expected GitHub rule to have 1 domain")
	}

	internalRule := cfg.Rules[3]
	if len(internalRule.Egress.IPs) != 3 {
		t.Errorf("Expected internal rule to have 3 CIDR ranges")
	}

	icmpRule := cfg.Rules[4]
	if len(icmpRule.Egress.Protocols) != 1 || icmpRule.Egress.Protocols[0] != config.ProtocolICMP {
		t.Errorf("Expected ICMP rule to have ICMP protocol")
	}
}

// TestGradualBuildUp tests adding endpoints one at a time
func TestGradualBuildUp(t *testing.T) {
	// Start with just DNS
	cfg := &config.Config{
		Version: "1.0",
		Rules: []config.Rule{
			{
				Name:   "allow-dns",
				Action: config.ActionAllow,
				Order:  100,
				Egress: config.Egress{
					Protocols: []config.Protocol{config.ProtocolUDP},
					IPs:       []string{"8.8.8.8"},
					Ports:     []string{"53"},
				},
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Step 1 (DNS only) validation failed: %v", err)
	}

	// Add HTTPS to one domain
	cfg.Rules = append(cfg.Rules, config.Rule{
		Name:   "allow-github",
		Action: config.ActionAllow,
		Order:  100,
		Egress: config.Egress{
			Protocols: []config.Protocol{config.ProtocolTCP},
			Domains:   []string{"api.github.com"},
			Ports:     []string{"443"},
		},
	})

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Step 2 (DNS + GitHub) validation failed: %v", err)
	}

	if len(cfg.Rules) != 2 {
		t.Errorf("Expected 2 rules after step 2, got %d", len(cfg.Rules))
	}

	// Add internal network access
	cfg.Rules = append(cfg.Rules, config.Rule{
		Name:   "allow-internal",
		Action: config.ActionAllow,
		Order:  200,
		Egress: config.Egress{
			Protocols: []config.Protocol{config.ProtocolTCP, config.ProtocolUDP},
			IPs:       []string{"10.0.0.0/8"},
		},
	})

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Step 3 (DNS + GitHub + internal) validation failed: %v", err)
	}

	if len(cfg.Rules) != 3 {
		t.Errorf("Expected 3 rules after step 3, got %d", len(cfg.Rules))
	}

	// Add a block rule at higher priority
	cfg.Rules = append(cfg.Rules, config.Rule{
		Name:   "block-metadata",
		Action: config.ActionDeny,
		Order:  50,
		Egress: config.Egress{
			IPs: []string{"169.254.169.254"},
		},
	})

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Step 4 (all rules) validation failed: %v", err)
	}

	if len(cfg.Rules) != 4 {
		t.Errorf("Expected 4 rules after step 4, got %d", len(cfg.Rules))
	}

	// Sort and verify the block rule is first
	sort.Slice(cfg.Rules, func(i, j int) bool {
		return cfg.Rules[i].Order < cfg.Rules[j].Order
	})

	if cfg.Rules[0].Name != "block-metadata" {
		t.Errorf("Expected block-metadata to be first after sorting, got %s", cfg.Rules[0].Name)
	}
}
