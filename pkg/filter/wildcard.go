package filter

import "strings"

// MatchWildcard checks if a domain matches a wildcard pattern
// Supports patterns like:
// - *.example.com matches foo.example.com, bar.example.com
// - example.com matches exactly example.com
func MatchWildcard(pattern, domain string) bool {
	// Exact match
	if pattern == domain {
		return true
	}

	// No wildcard in pattern
	if !strings.HasPrefix(pattern, "*.") {
		return false
	}

	// Extract the suffix after "*."
	suffix := pattern[2:] // Remove "*."

	// Domain must end with the suffix
	if !strings.HasSuffix(domain, suffix) {
		return false
	}

	// Make sure it's a proper subdomain match, not partial
	// e.g., *.example.com should match foo.example.com but not fooexample.com
	if len(domain) == len(suffix) {
		return true // Exact match of suffix
	}

	// Check that there's a dot before the suffix
	beforeSuffix := domain[:len(domain)-len(suffix)]
	return strings.HasSuffix(beforeSuffix, ".")
}
