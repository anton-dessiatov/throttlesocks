package main

import (
	"fmt"
	"strconv"
	"strings"
)

// uom stands for Unit Of Measurement. Units are BITS per second, not bytes
var uomSuffixes = []struct {
	unit string
	mul  int64
	div  int64
}{
	// IPerf assumes that megabit per second is exactly 1000000 bits per second
	// (not 1024 * 1024)
	{unit: "Kbps", mul: 1000, div: 8},
	{unit: "Mbps", mul: 1000 * 1000, div: 8},
	{unit: "Gbps", mul: 1000 * 1000 * 1000, div: 8},
	// tcptrack, on the other hand, uses <prefix>bytes per second where prefix
	// is a power of 2, that's why I'm using powers of 1024 for bytes-per-second
	// units
	{unit: "KBps", mul: 1024, div: 1},
	{unit: "MBps", mul: 1024 * 1024, div: 1},
	{unit: "GBps", mul: 1024 * 1024 * 1024, div: 1},
	{unit: "bps", mul: 1, div: 8},
	{unit: "Bps", mul: 1, div: 1},
}

// Tries to parse an UOM suffix from a string. Returns string stripped from that
// suffix and a multiplier. If no suffix matches, returns string as is and 1 as
// a multiplier.
func parseSuffix(s string) (string, int64, int64) {
	for _, v := range uomSuffixes {
		if strings.HasSuffix(s, v.unit) {
			return s[0 : len(s)-len(v.unit)], v.mul, v.div
		}
	}

	return s, 1, 1
}

// ParseLimit parses given limit string to bytes per second.
func ParseLimit(s string) (int64, error) {
	numberString, mul, div := parseSuffix(s)
	bytesPerSecond, err := strconv.ParseInt(numberString, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("Failed to parse %q", s)
	}
	bytesPerSecond *= mul
	bytesPerSecond /= div

	if bytesPerSecond < 0 {
		return 0, fmt.Errorf("Negative values are not accepted as a bandwidth limit (%q)", s)
	}

	return bytesPerSecond, nil
}
