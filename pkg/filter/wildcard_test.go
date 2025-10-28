package filter

import "testing"

// TestWildcardMatching tests wildcard domain matching
func TestWildcardMatching(t *testing.T) {
	testCases := []struct {
		name    string
		pattern string
		domain  string
		want    bool
	}{
		{
			name:    "exact match",
			pattern: "example.com",
			domain:  "example.com",
			want:    true,
		},
		{
			name:    "exact match no wildcard",
			pattern: "example.com",
			domain:  "www.example.com",
			want:    false,
		},
		{
			name:    "wildcard subdomain",
			pattern: "*.example.com",
			domain:  "www.example.com",
			want:    true,
		},
		{
			name:    "wildcard api subdomain",
			pattern: "*.example.com",
			domain:  "api.example.com",
			want:    true,
		},
		{
			name:    "wildcard matches base domain",
			pattern: "*.example.com",
			domain:  "example.com",
			want:    true,
		},
		{
			name:    "wildcard matches nested subdomains",
			pattern: "*.example.com",
			domain:  "test.api.example.com",
			want:    true,
		},
		{
			name:    "wildcard cdn match",
			pattern: "*.cdn.com",
			domain:  "edge.cdn.com",
			want:    true,
		},
		{
			name:    "wildcard no match different domain",
			pattern: "*.cdn.com",
			domain:  "other.com",
			want:    false,
		},
		{
			name:    "wildcard cloudfront",
			pattern: "*.cloudfront.net",
			domain:  "d123.cloudfront.net",
			want:    true,
		},
		{
			name:    "no wildcard no match",
			pattern: "github.com",
			domain:  "api.github.com",
			want:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := MatchWildcard(tc.pattern, tc.domain)
			if got != tc.want {
				t.Errorf("MatchWildcard(%q, %q) = %v, want %v", tc.pattern, tc.domain, got, tc.want)
			}
		})
	}
}
