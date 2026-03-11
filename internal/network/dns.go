package network

import (
	"fmt"
	"net"
	"time"

	"github.com/miekg/dns"
)

// MinTTLFloor is the minimum TTL in seconds to prevent overly aggressive refreshes
const MinTTLFloor uint32 = 60

// DNSResult holds the result of a DNS query including TTL information
type DNSResult struct {
	IPs []string
	TTL uint32 // seconds
}

// QueryDNS performs an A record lookup for the given domain and returns IPs with TTL.
// It reads the system nameserver from /etc/resolv.conf and queries via miekg/dns.
func QueryDNS(domain string) (*DNSResult, error) {
	// Ensure domain has trailing dot for DNS query
	if domain[len(domain)-1] != '.' {
		domain = domain + "."
	}

	// Read system nameserver
	conf, err := dns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil {
		return nil, fmt.Errorf("failed to read resolv.conf: %w", err)
	}

	if len(conf.Servers) == 0 {
		return nil, fmt.Errorf("no nameservers found in resolv.conf")
	}

	// Build A record query
	msg := new(dns.Msg)
	msg.SetQuestion(domain, dns.TypeA)
	msg.RecursionDesired = true

	// Create client with timeout
	client := &dns.Client{
		Timeout: 5 * time.Second,
	}

	// Try each nameserver until one succeeds
	var lastErr error
	for _, server := range conf.Servers {
		serverAddr := net.JoinHostPort(server, conf.Port)
		resp, _, err := client.Exchange(msg, serverAddr)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.Rcode != dns.RcodeSuccess {
			lastErr = fmt.Errorf("DNS query failed with rcode %d", resp.Rcode)
			continue
		}

		return parseARecords(resp)
	}

	if lastErr != nil {
		return nil, fmt.Errorf("all nameservers failed, last error: %w", lastErr)
	}

	return nil, fmt.Errorf("no nameservers available")
}

// parseARecords extracts IPv4 addresses and minimum TTL from DNS answer records
func parseARecords(resp *dns.Msg) (*DNSResult, error) {
	result := &DNSResult{}
	var minTTL uint32
	first := true

	for _, answer := range resp.Answer {
		if a, ok := answer.(*dns.A); ok {
			result.IPs = append(result.IPs, a.A.String())
			ttl := answer.Header().Ttl
			if first || ttl < minTTL {
				minTTL = ttl
				first = false
			}
		}
	}

	if len(result.IPs) == 0 {
		return nil, fmt.Errorf("no A records found in DNS response")
	}

	// Enforce TTL floor
	if minTTL < MinTTLFloor {
		minTTL = MinTTLFloor
	}

	result.TTL = minTTL
	return result, nil
}
