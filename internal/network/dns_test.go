package network

import (
	"testing"

	"github.com/miekg/dns"
)

func TestQueryDNS_RealDomain(t *testing.T) {
	result, err := QueryDNS("example.com")
	if err != nil {
		t.Skipf("DNS resolution not available: %v", err)
	}

	if len(result.IPs) == 0 {
		t.Error("QueryDNS(\"example.com\") returned no IPs")
	}

	// TTL should be at least MinTTLFloor due to floor enforcement
	if result.TTL < MinTTLFloor {
		t.Errorf("TTL %d is below floor %d", result.TTL, MinTTLFloor)
	}
}

func TestQueryDNS_NonexistentDomain(t *testing.T) {
	_, err := QueryDNS("this-domain-does-not-exist-12345.example.invalid")
	if err == nil {
		t.Error("Expected error for nonexistent domain, got nil")
	}
}

func TestQueryDNS_TrailingDot(t *testing.T) {
	// Domain with trailing dot should also work
	result, err := QueryDNS("example.com.")
	if err != nil {
		t.Skipf("DNS resolution not available: %v", err)
	}

	if len(result.IPs) == 0 {
		t.Error("QueryDNS(\"example.com.\") returned no IPs")
	}
}

func TestParseARecords_TTLFloor(t *testing.T) {
	// Create a mock DNS response with a very low TTL
	resp := &dns.Msg{
		Answer: []dns.RR{
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    5, // Very low TTL
				},
				A: []byte{93, 184, 216, 34},
			},
		},
	}

	result, err := parseARecords(resp)
	if err != nil {
		t.Fatalf("parseARecords failed: %v", err)
	}

	if result.TTL != MinTTLFloor {
		t.Errorf("Expected TTL to be clamped to %d, got %d", MinTTLFloor, result.TTL)
	}
}

func TestParseARecords_MinTTLAcrossRecords(t *testing.T) {
	resp := &dns.Msg{
		Answer: []dns.RR{
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    300,
				},
				A: []byte{93, 184, 216, 34},
			},
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    120,
				},
				A: []byte{93, 184, 216, 35},
			},
		},
	}

	result, err := parseARecords(resp)
	if err != nil {
		t.Fatalf("parseARecords failed: %v", err)
	}

	if result.TTL != 120 {
		t.Errorf("Expected minimum TTL 120, got %d", result.TTL)
	}

	if len(result.IPs) != 2 {
		t.Errorf("Expected 2 IPs, got %d", len(result.IPs))
	}
}

func TestParseARecords_NoARecords(t *testing.T) {
	resp := &dns.Msg{
		Answer: []dns.RR{}, // Empty answer
	}

	_, err := parseARecords(resp)
	if err == nil {
		t.Error("Expected error for empty answer, got nil")
	}
}
