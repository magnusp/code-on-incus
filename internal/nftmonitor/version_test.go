package nftmonitor

import (
	"strings"
	"testing"
)

func TestParseNFTVersion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		major   int
		minor   int
		patch   int
		wantErr bool
	}{
		{"standard version", "1.0.9", 1, 0, 9, false},
		{"minimum version", "0.9.0", 0, 9, 0, false},
		{"old version", "0.8.3", 0, 8, 3, false},
		{"two-part version", "1.0", 1, 0, 0, false},
		{"with whitespace", "  1.0.2  ", 1, 0, 2, false},
		{"empty string", "", 0, 0, 0, true},
		{"no minor", "1", 0, 0, 0, true},
		{"invalid major", "abc.1.0", 0, 0, 0, true},
		{"invalid minor", "1.abc.0", 0, 0, 0, true},
		{"invalid patch", "1.0.abc", 0, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := ParseNFTVersion(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if v.Major != tt.major {
				t.Errorf("major = %d, want %d", v.Major, tt.major)
			}
			if v.Minor != tt.minor {
				t.Errorf("minor = %d, want %d", v.Minor, tt.minor)
			}
			if v.Patch != tt.patch {
				t.Errorf("patch = %d, want %d", v.Patch, tt.patch)
			}
		})
	}
}

func TestExtractNFTVersion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			"standard output",
			"nftables v1.0.9 (Old Doc Yak #3)",
			"1.0.9",
			false,
		},
		{
			"older format",
			"nftables v0.9.3 (Topsy)",
			"0.9.3",
			false,
		},
		{
			"empty string",
			"",
			"",
			true,
		},
		{
			"no version marker",
			"some random output",
			"",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractNFTVersion(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMeetsMinimumNFTVersion(t *testing.T) {
	tests := []struct {
		name  string
		major int
		minor int
		patch int
		want  bool
	}{
		{"exact minimum", 0, 9, 0, true},
		{"above minimum minor", 0, 9, 3, true},
		{"above minimum major", 1, 0, 0, true},
		{"standard modern", 1, 0, 9, true},
		{"below minimum", 0, 8, 3, false},
		{"way below", 0, 7, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &NFTVersion{Major: tt.major, Minor: tt.minor, Patch: tt.patch, Raw: "test"}
			got := MeetsMinimumNFTVersion(v)
			if got != tt.want {
				t.Errorf("MeetsMinimumNFTVersion(%d.%d.%d) = %v, want %v",
					tt.major, tt.minor, tt.patch, got, tt.want)
			}
		})
	}
}

func TestFormatMinNFTVersionError(t *testing.T) {
	v := &NFTVersion{Major: 0, Minor: 8, Patch: 3, Raw: "0.8.3"}
	msg := FormatMinNFTVersionError(v)

	if !strings.Contains(msg, "0.8.3") {
		t.Error("message should contain the actual version")
	}
	if !strings.Contains(msg, "0.9.0") {
		t.Error("message should contain the minimum version")
	}
	if !strings.Contains(msg, "nftables") {
		t.Error("message should mention nftables")
	}
}
