package dns

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

const (
	refreshInterval = 5 * time.Minute
	dnsCacheTTL     = 5 * time.Minute
)

// Resolver handles DNS resolution and caching
type Resolver struct {
	cache    map[string]*cacheEntry
	cacheMu  sync.RWMutex
	client   *dns.Client
	servers  []string
}

type cacheEntry struct {
	ips       []string
	expiresAt time.Time
}

// NewResolver creates a new DNS resolver
func NewResolver() (*Resolver, error) {
	return &Resolver{
		cache: make(map[string]*cacheEntry),
		client: &dns.Client{
			Timeout: 5 * time.Second,
		},
		servers: []string{"8.8.8.8:53", "1.1.1.1:53"},
	}, nil
}

// Resolve resolves a domain name to IP addresses
func (r *Resolver) Resolve(domain string) ([]string, error) {
	// Check cache first
	r.cacheMu.RLock()
	if entry, ok := r.cache[domain]; ok && time.Now().Before(entry.expiresAt) {
		ips := make([]string, len(entry.ips))
		copy(ips, entry.ips)
		r.cacheMu.RUnlock()
		return ips, nil
	}
	r.cacheMu.RUnlock()

	// Handle wildcard domains (*.example.com)
	if strings.HasPrefix(domain, "*.") {
		// For wildcards, we can't pre-resolve
		// We'll need to handle this differently (e.g., during connection time)
		log.Printf("Wildcard domain %s requires runtime resolution", domain)
		return nil, fmt.Errorf("wildcard domains not yet supported in DNS pre-resolution")
	}

	// Perform DNS lookup
	ips, err := r.lookup(domain)
	if err != nil {
		return nil, err
	}

	// Update cache
	r.cacheMu.Lock()
	r.cache[domain] = &cacheEntry{
		ips:       ips,
		expiresAt: time.Now().Add(dnsCacheTTL),
	}
	r.cacheMu.Unlock()

	return ips, nil
}

// lookup performs the actual DNS query
func (r *Resolver) lookup(domain string) ([]string, error) {
	var allIPs []string

	// Try A records (IPv4)
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(domain), dns.TypeA)

	for _, server := range r.servers {
		resp, _, err := r.client.Exchange(msg, server)
		if err != nil {
			log.Printf("DNS query to %s failed: %v", server, err)
			continue
		}

		for _, answer := range resp.Answer {
			if a, ok := answer.(*dns.A); ok {
				allIPs = append(allIPs, a.A.String())
			}
		}

		if len(allIPs) > 0 {
			break // Success
		}
	}

	// Fallback to system resolver
	if len(allIPs) == 0 {
		addrs, err := net.LookupHost(domain)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve %s: %w", domain, err)
		}
		allIPs = addrs
	}

	if len(allIPs) == 0 {
		return nil, fmt.Errorf("no IPs found for domain %s", domain)
	}

	return allIPs, nil
}

// StartPeriodicRefresh starts periodic DNS cache refresh
func (r *Resolver) StartPeriodicRefresh(stopChan <-chan struct{}, callback func(string, []string)) {
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.refreshCache(callback)
		case <-stopChan:
			return
		}
	}
}

// refreshCache refreshes all cached DNS entries
func (r *Resolver) refreshCache(callback func(string, []string)) {
	r.cacheMu.RLock()
	domains := make([]string, 0, len(r.cache))
	for domain := range r.cache {
		domains = append(domains, domain)
	}
	r.cacheMu.RUnlock()

	for _, domain := range domains {
		ips, err := r.lookup(domain)
		if err != nil {
			log.Printf("Failed to refresh DNS for %s: %v", domain, err)
			continue
		}

		// Update cache
		r.cacheMu.Lock()
		r.cache[domain] = &cacheEntry{
			ips:       ips,
			expiresAt: time.Now().Add(dnsCacheTTL),
		}
		r.cacheMu.Unlock()

		// Notify callback
		if callback != nil {
			callback(domain, ips)
		}
	}
}
