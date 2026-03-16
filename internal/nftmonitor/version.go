package nftmonitor

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	// MinNFTVersionMajor is the minimum required nftables major version
	MinNFTVersionMajor = 0
	// MinNFTVersionMinor is the minimum required nftables minor version
	MinNFTVersionMinor = 9
	// MinNFTVersionPatch is the minimum required nftables patch version
	MinNFTVersionPatch = 0
)

// NFTVersion represents a parsed nftables version
type NFTVersion struct {
	Major int
	Minor int
	Patch int
	Raw   string
}

// ParseNFTVersion parses a version string like "1.0.9", "0.9.3", etc.
func ParseNFTVersion(str string) (*NFTVersion, error) {
	str = strings.TrimSpace(str)
	if str == "" {
		return nil, fmt.Errorf("empty version string")
	}

	parts := strings.SplitN(str, ".", 4)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid version format: %q (expected major.minor)", str)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid major version in %q: %w", str, err)
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid minor version in %q: %w", str, err)
	}

	patch := 0
	if len(parts) >= 3 {
		patch, err = strconv.Atoi(parts[2])
		if err != nil {
			return nil, fmt.Errorf("invalid patch version in %q: %w", str, err)
		}
	}

	return &NFTVersion{
		Major: major,
		Minor: minor,
		Patch: patch,
		Raw:   str,
	}, nil
}

// ExtractNFTVersion extracts the version string from `nft --version` output.
// Example output: "nftables v1.0.9 (Old Doc Yak #3)"
func ExtractNFTVersion(output string) (string, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return "", fmt.Errorf("empty version output")
	}

	// Look for "v" prefix in the output: "nftables v1.0.9 (...)"
	for _, field := range strings.Fields(output) {
		if strings.HasPrefix(field, "v") {
			return strings.TrimPrefix(field, "v"), nil
		}
	}

	return "", fmt.Errorf("could not find version in output: %q", output)
}

// MeetsMinimumNFTVersion checks if the given version meets the minimum requirement (>= 0.9.0)
func MeetsMinimumNFTVersion(v *NFTVersion) bool {
	if v.Major > MinNFTVersionMajor {
		return true
	}
	if v.Major == MinNFTVersionMajor {
		if v.Minor > MinNFTVersionMinor {
			return true
		}
		if v.Minor == MinNFTVersionMinor {
			return v.Patch >= MinNFTVersionPatch
		}
	}
	return false
}

// FormatMinNFTVersionError returns an actionable error message for users with old nftables versions
func FormatMinNFTVersionError(v *NFTVersion) string {
	return fmt.Sprintf(
		"nftables version %s is below the minimum required %d.%d.%d. "+
			"Older versions may lack required features for network monitoring. "+
			"Please upgrade nftables (e.g., sudo apt install nftables)",
		v.Raw, MinNFTVersionMajor, MinNFTVersionMinor, MinNFTVersionPatch,
	)
}
