package filter

import (
	"fmt"
	"log"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/skaegi/legion-router/pkg/config"
	"github.com/skaegi/legion-router/pkg/dns"
	"github.com/skaegi/legion-router/pkg/nftables"
)

// Filter manages the egress filtering
type Filter struct {
	config     *config.Config
	configPath string
	dns        *dns.Resolver
	nft        *nftables.Manager
	mu         sync.RWMutex
	stopChan   chan struct{}
	watcher    *fsnotify.Watcher
}

// New creates a new Filter instance
func New(cfg *config.Config, configPath string) (*Filter, error) {
	// Create DNS resolver
	resolver, err := dns.NewResolver()
	if err != nil {
		return nil, fmt.Errorf("failed to create DNS resolver: %w", err)
	}

	// Create nftables manager
	nftMgr, err := nftables.NewManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create nftables manager: %w", err)
	}

	// Create file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	return &Filter{
		config:     cfg,
		configPath: configPath,
		dns:        resolver,
		nft:        nftMgr,
		stopChan:   make(chan struct{}),
		watcher:    watcher,
	}, nil
}

// Start begins the filtering process
func (f *Filter) Start() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	log.Println("Setting up nftables rules...")
	if err := f.nft.Setup(); err != nil {
		return fmt.Errorf("failed to setup nftables: %w", err)
	}

	log.Println("Processing filtering rules...")
	if err := f.applyRules(); err != nil {
		return fmt.Errorf("failed to apply rules: %w", err)
	}

	// Start watching config file for changes
	if err := f.watcher.Add(f.configPath); err != nil {
		log.Printf("Warning: failed to watch config file: %v", err)
	} else {
		log.Printf("Watching config file for changes: %s", f.configPath)
		go f.watchConfigFile()
	}

	// Start DNS resolver background tasks
	go f.dns.StartPeriodicRefresh(f.stopChan, func(domain string, ips []string) {
		// Callback when DNS entries are refreshed
		if err := f.updateDomainIPs(domain, ips); err != nil {
			log.Printf("Failed to update IPs for domain %s: %v", domain, err)
		}
	})

	return nil
}

// Stop stops the filter
func (f *Filter) Stop() error {
	close(f.stopChan)

	if f.watcher != nil {
		f.watcher.Close()
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	log.Println("Cleaning up nftables rules...")
	return f.nft.Cleanup()
}

// applyRules processes all configuration rules and applies them
func (f *Filter) applyRules() error {
	for _, rule := range f.config.Rules {
		if err := f.applyRule(rule); err != nil {
			return fmt.Errorf("failed to apply rule %s: %w", rule.Name, err)
		}
		log.Printf("Applied rule: %s (order: %d, action: %s)", rule.Name, rule.Order, rule.Action)
	}
	return nil
}

// applyRule applies a single rule
func (f *Filter) applyRule(rule config.Rule) error {
	// Handle domain-based rules
	if len(rule.Egress.Domains) > 0 {
		for _, domain := range rule.Egress.Domains {
			// Skip wildcard domains - they can't be pre-resolved
			// Wildcard matching would need to be done at connection time with SNI inspection
			// For now, wildcards are logged but not enforced
			if isWildcard(domain) {
				log.Printf("Note: Wildcard domain %s in rule %s - wildcard enforcement requires SNI inspection (Phase 2)", domain, rule.Name)
				continue
			}

			// Resolve domain to IPs
			ips, err := f.dns.Resolve(domain)
			if err != nil {
				log.Printf("Warning: failed to resolve domain %s: %v", domain, err)
				continue
			}

			// Add IPs to nftables
			if err := f.nft.AddRule(nftables.Rule{
				Name:      rule.Name,
				Action:    string(rule.Action),
				Priority:  rule.Order,
				IPs:       ips,
				Ports:     rule.Egress.Ports,
				Protocols: protocolsToStrings(rule.Egress.Protocols),
			}); err != nil {
				return fmt.Errorf("failed to add nftables rule for domain %s: %w", domain, err)
			}
		}
	}

	// Handle IP-based rules
	if len(rule.Egress.IPs) > 0 {
		if err := f.nft.AddRule(nftables.Rule{
			Name:      rule.Name,
			Action:    string(rule.Action),
			Priority:  rule.Order,
			IPs:       rule.Egress.IPs,
			Ports:     rule.Egress.Ports,
			Protocols: protocolsToStrings(rule.Egress.Protocols),
		}); err != nil {
			return fmt.Errorf("failed to add nftables rule: %w", err)
		}
	}

	// Handle protocol-only rules (e.g., allow all ICMP)
	if len(rule.Egress.Protocols) > 0 && len(rule.Egress.IPs) == 0 && len(rule.Egress.Domains) == 0 {
		if err := f.nft.AddRule(nftables.Rule{
			Name:      rule.Name,
			Action:    string(rule.Action),
			Priority:  rule.Order,
			Ports:     rule.Egress.Ports,
			Protocols: protocolsToStrings(rule.Egress.Protocols),
		}); err != nil {
			return fmt.Errorf("failed to add protocol rule: %w", err)
		}
	}

	return nil
}

// updateDomainIPs updates nftables rules when DNS entries change
func (f *Filter) updateDomainIPs(domain string, ips []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Find rules that use this domain
	for _, rule := range f.config.Rules {
		for _, d := range rule.Egress.Domains {
			if matchDomain(d, domain) {
				// Update the rule with new IPs
				if err := f.nft.UpdateIPs(rule.Name, ips); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// matchDomain checks if a domain pattern matches a domain
func matchDomain(pattern, domain string) bool {
	return MatchWildcard(pattern, domain)
}

// isWildcard checks if a domain pattern is a wildcard
func isWildcard(domain string) bool {
	return len(domain) > 2 && domain[0] == '*' && domain[1] == '.'
}

// protocolsToStrings converts Protocol enums to strings
func protocolsToStrings(protocols []config.Protocol) []string {
	result := make([]string, len(protocols))
	for i, p := range protocols {
		result[i] = string(p)
	}
	return result
}

// watchConfigFile watches for config file changes and reloads
func (f *Filter) watchConfigFile() {
	for {
		select {
		case event, ok := <-f.watcher.Events:
			if !ok {
				return
			}

			// Respond to write and create events (editors often write to temp files)
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				log.Printf("Config file changed, reloading: %s", event.Name)
				if err := f.reloadConfig(); err != nil {
					log.Printf("Error reloading config: %v", err)
				} else {
					log.Println("Config reloaded successfully")
				}
			}

		case err, ok := <-f.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Config file watcher error: %v", err)

		case <-f.stopChan:
			return
		}
	}
}

// reloadConfig reloads and applies the configuration
func (f *Filter) reloadConfig() error {
	// Load new config
	newConfig, err := config.Load(f.configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Validate config
	if err := newConfig.Validate(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	log.Println("Clearing existing nftables rules...")
	// Clear existing rules and recreate
	if err := f.nft.Cleanup(); err != nil {
		return fmt.Errorf("failed to cleanup nftables: %w", err)
	}

	if err := f.nft.Setup(); err != nil {
		return fmt.Errorf("failed to setup nftables: %w", err)
	}

	// Update config
	f.config = newConfig

	// Apply new rules
	log.Println("Applying new configuration rules...")
	if err := f.applyRules(); err != nil {
		return fmt.Errorf("failed to apply rules: %w", err)
	}

	return nil
}
