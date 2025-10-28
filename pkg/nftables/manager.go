package nftables

import (
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
)

const (
	tableName  = "legion_filter"
	chainName  = "egress_filter"
	setNameFmt = "ips_%s" // IP sets per rule
)

// Manager manages nftables rules
type Manager struct {
	conn  *nftables.Conn
	table *nftables.Table
	chain *nftables.Chain
	sets  map[string]*nftables.Set // Rule name -> IP set
}

// Rule represents a filtering rule to be applied
type Rule struct {
	Name      string
	Action    string   // "allow" or "deny"
	Priority  int      // Lower = higher priority
	IPs       []string // IP addresses or CIDR ranges
	Ports     []string // Port numbers or ranges
	Protocols []string // tcp, udp, icmp
}

// NewManager creates a new nftables manager
func NewManager() (*Manager, error) {
	conn, err := nftables.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create nftables connection: %w", err)
	}

	return &Manager{
		conn: conn,
		sets: make(map[string]*nftables.Set),
	}, nil
}

// Setup initializes the nftables table and chain
func (m *Manager) Setup() error {
	// Create table
	m.table = m.conn.AddTable(&nftables.Table{
		Family: nftables.TableFamilyIPv4,
		Name:   tableName,
	})

	// Create chain for forward filtering (traffic passing through the router)
	// Note: Default policy will be DROP - any unmatched traffic is dropped
	m.chain = m.conn.AddChain(&nftables.Chain{
		Name:     chainName,
		Table:    m.table,
		Type:     nftables.ChainTypeFilter,
		Hooknum:  nftables.ChainHookForward,
		Priority: nftables.ChainPriorityFilter,
	})

	// Add NAT chain for masquerading outbound traffic
	natChain := m.conn.AddChain(&nftables.Chain{
		Name:     "postrouting",
		Table:    m.table,
		Type:     nftables.ChainTypeNAT,
		Hooknum:  nftables.ChainHookPostrouting,
		Priority: nftables.ChainPriorityNATSource,
	})

	// Add masquerade rule for all outbound traffic
	m.conn.AddRule(&nftables.Rule{
		Table: m.table,
		Chain: natChain,
		Exprs: []expr.Any{
			&expr.Masq{},
		},
	})

	// Add default drop rule at the end of the chain
	// Any traffic that didn't match any rules will be dropped
	m.conn.AddRule(&nftables.Rule{
		Table: m.table,
		Chain: m.chain,
		Exprs: []expr.Any{
			&expr.Verdict{Kind: expr.VerdictDrop},
		},
	})

	// Flush and apply
	if err := m.conn.Flush(); err != nil {
		return fmt.Errorf("failed to flush nftables: %w", err)
	}

	log.Printf("Created nftables table '%s' with forward chain and NAT", tableName)
	return nil
}

// Cleanup removes all nftables rules
func (m *Manager) Cleanup() error {
	if m.table != nil {
		m.conn.DelTable(m.table)
	}

	return m.conn.Flush()
}

// AddRule adds a new filtering rule
func (m *Manager) AddRule(rule Rule) error {
	// Create IP set for this rule if there are IPs
	var ipSet *nftables.Set
	if len(rule.IPs) > 0 {
		setName := fmt.Sprintf(setNameFmt, sanitizeName(rule.Name))
		ipSet = &nftables.Set{
			Table:   m.table,
			Name:    setName,
			KeyType: nftables.TypeIPAddr,
		}

		if err := m.conn.AddSet(ipSet, nil); err != nil {
			return fmt.Errorf("failed to create IP set: %w", err)
		}

		m.sets[rule.Name] = ipSet

		// Add IPs to set
		if err := m.addIPsToSet(ipSet, rule.IPs); err != nil {
			return fmt.Errorf("failed to add IPs to set: %w", err)
		}
	}

	// Build nftables rule expressions
	exprs, err := m.buildRuleExpressions(rule, ipSet)
	if err != nil {
		return fmt.Errorf("failed to build rule expressions: %w", err)
	}

	// Add the rule
	m.conn.AddRule(&nftables.Rule{
		Table: m.table,
		Chain: m.chain,
		Exprs: exprs,
	})

	// Apply changes
	if err := m.conn.Flush(); err != nil {
		return fmt.Errorf("failed to apply nftables rule: %w", err)
	}

	return nil
}

// buildRuleExpressions builds nftables expressions for a rule
func (m *Manager) buildRuleExpressions(rule Rule, ipSet *nftables.Set) ([]expr.Any, error) {
	var exprs []expr.Any

	// Match protocol if specified
	if len(rule.Protocols) > 0 {
		for _, proto := range rule.Protocols {
			protoNum := protocolToNum(proto)
			exprs = append(exprs,
				// Load protocol from IP header
				&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1},
				// Compare with desired protocol
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: 1,
					Data:     []byte{protoNum},
				},
			)
		}
	}

	// Match destination IP if IP set exists
	if ipSet != nil {
		exprs = append(exprs,
			// Load destination IP
			&expr.Payload{
				DestRegister: 1,
				Base:         expr.PayloadBaseNetworkHeader,
				Offset:       16, // Destination IP offset in IPv4 header
				Len:          4,  // IPv4 address length
			},
			// Check if IP is in set
			&expr.Lookup{
				SourceRegister: 1,
				SetName:        ipSet.Name,
				SetID:          ipSet.ID,
			},
		)
	}

	// Match destination port if specified
	if len(rule.Ports) > 0 {
		for _, portStr := range rule.Ports {
			portExprs, err := buildPortExpression(portStr)
			if err != nil {
				log.Printf("Warning: invalid port specification %s: %v", portStr, err)
				continue
			}
			exprs = append(exprs, portExprs...)
		}
	}

	// Add verdict (accept or drop)
	if rule.Action == "allow" {
		exprs = append(exprs, &expr.Verdict{Kind: expr.VerdictAccept})
	} else {
		exprs = append(exprs, &expr.Verdict{Kind: expr.VerdictDrop})
	}

	return exprs, nil
}

// addIPsToSet adds IP addresses to an nftables set
func (m *Manager) addIPsToSet(set *nftables.Set, ips []string) error {
	for _, ipStr := range ips {
		// Handle CIDR notation
		if _, ipNet, err := net.ParseCIDR(ipStr); err == nil {
			// For CIDR, add the network address
			// TODO: Properly handle CIDR ranges in sets
			ip := ipNet.IP.To4()
			if ip != nil {
				if err := m.conn.SetAddElements(set, []nftables.SetElement{{Key: ip}}); err != nil {
					return err
				}
			}
		} else {
			// Single IP
			ip := net.ParseIP(ipStr).To4()
			if ip == nil {
				log.Printf("Warning: invalid IP address: %s", ipStr)
				continue
			}

			if err := m.conn.SetAddElements(set, []nftables.SetElement{{Key: ip}}); err != nil {
				return err
			}
		}
	}

	return nil
}

// UpdateIPs updates the IPs in a rule's set
func (m *Manager) UpdateIPs(ruleName string, ips []string) error {
	set, ok := m.sets[ruleName]
	if !ok {
		return fmt.Errorf("no IP set found for rule %s", ruleName)
	}

	// Flush existing elements
	// TODO: Implement proper set update without flushing

	// Add new IPs
	return m.addIPsToSet(set, ips)
}

// buildPortExpression builds nftables expressions for port matching
// Supports both single ports (443) and ranges (8000-9000)
func buildPortExpression(portStr string) ([]expr.Any, error) {
	var exprs []expr.Any

	// Check if it's a range (contains "-")
	if idx := strings.Index(portStr, "-"); idx > 0 {
		// Port range
		var startPort, endPort uint16
		if _, err := fmt.Sscanf(portStr, "%d-%d", &startPort, &endPort); err != nil {
			return nil, fmt.Errorf("invalid port range: %s", portStr)
		}

		if startPort > endPort {
			return nil, fmt.Errorf("invalid port range %s: start > end", portStr)
		}

		exprs = append(exprs,
			// Load destination port
			&expr.Payload{
				DestRegister: 1,
				Base:         expr.PayloadBaseTransportHeader,
				Offset:       2, // Destination port offset
				Len:          2, // Port length
			},
			// Check if port is in range [startPort, endPort]
			&expr.Range{
				Op:       expr.CmpOpEq,
				Register: 1,
				FromData: []byte{byte(startPort >> 8), byte(startPort & 0xff)},
				ToData:   []byte{byte(endPort >> 8), byte(endPort & 0xff)},
			},
		)
	} else {
		// Single port
		var port uint16
		if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
			return nil, fmt.Errorf("invalid port: %s", portStr)
		}

		exprs = append(exprs,
			// Load destination port
			&expr.Payload{
				DestRegister: 1,
				Base:         expr.PayloadBaseTransportHeader,
				Offset:       2, // Destination port offset
				Len:          2, // Port length
			},
			// Compare port
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     []byte{byte(port >> 8), byte(port & 0xff)},
			},
		)
	}

	return exprs, nil
}

// protocolToNum converts protocol name to number
func protocolToNum(proto string) uint8 {
	switch proto {
	case "tcp":
		return unix.IPPROTO_TCP
	case "udp":
		return unix.IPPROTO_UDP
	case "icmp":
		return unix.IPPROTO_ICMP
	default:
		return 0
	}
}

// sanitizeName creates a valid nftables name
func sanitizeName(name string) string {
	// nftables names can't have certain characters
	// Replace invalid chars with underscore
	result := ""
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			result += string(c)
		} else {
			result += "_"
		}
	}
	return result
}
