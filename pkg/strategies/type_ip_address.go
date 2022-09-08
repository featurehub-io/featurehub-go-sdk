package strategies

import (
	"fmt"
	"net"
	"strings"
)

// TypeIPAddress is for ip-address values (eg "1.2.3.4" or "10.0.0.0/16"):
const TypeIPAddress = "IP_ADDRESS"

// parseIP get ip address from CIDR or simple ip (eg "1.2.3.4" or "10.0.0.0/16"):
func parseIP(value string) (net.IP, error) {
	// Check for ip contain CIDR
	isCIDR := strings.Contains(value, "/")

	if isCIDR {
		ip, _, err := net.ParseCIDR(value)
		return ip, err
	}

	// Try to parse simple ip address
	ip := net.ParseIP(value)
	if len(ip) == 0 {
		return net.IP{}, fmt.Errorf("unknown ip: %s", value)
	}

	return ip, nil
}

// IPAddress asserts the given parameters then passes on for evaluation:
func IPAddress(conditional string, options []interface{}, value interface{}) (bool, error) {

	// Type assert the value:
	assertedValue, ok := value.(string)
	if !ok {
		return false, fmt.Errorf("Unable to assert value (%v) as string", value)
	}

	ip, err := parseIP(assertedValue)
	if err != nil {
		return false, err
	}
	assertedValue = ip.String()

	// Type assert all of the options:
	var assertedOptions []string
	for _, option := range options {
		assertedOption, ok := option.(string)
		if !ok {
			return false, fmt.Errorf("Unable to assert value (%v) as string", option)
		}
		assertedOptions = append(assertedOptions, assertedOption)
	}

	// The string evaluations are fine for TypeIPAddress:
	return evaluateIPAddress(conditional, assertedOptions, assertedValue), nil
}

// evaluateIPAddress makes evaluations for TypeIPAddress values:
func evaluateIPAddress(conditional string, options []string, value string) bool {

	// Make sure we have a value:
	if len(value) == 0 {
		return false
	}

	switch conditional {

	case ConditionalEquals:
		// Return true if the value is equal to any of the options:
		for _, option := range options {
			if value == option {
				return true
			}
		}
		return false

	case ConditionalNotEquals:
		// Return false if the value is equal to any of the options:
		for _, option := range options {
			if value == option {
				return false
			}
		}
		return true

	case ConditionalExcludes:

		// Parse the value IP:
		valueIP, err := parseIP(value)
		if err != nil {
			return false
		}

		// Return false if the value is included by any of the options:
		for _, option := range options {

			// Parse each option address:
			_, optionNet, err := net.ParseCIDR(option)
			if err != nil {
				return false
			}

			if optionNet.Contains(valueIP) {
				return false
			}
		}
		return true

	case ConditionalIncludes:

		// Parse the value IP:
		valueIP, err := parseIP(value)
		if err != nil {
			return false
		}

		// Return true if the value is included by any of the options:
		for _, option := range options {

			// Parse each option address:
			_, optionNet, err := net.ParseCIDR(option)
			if err != nil {
				return false
			}

			if optionNet.Contains(valueIP) {
				return true
			}
		}
		return false

	default:
		return false
	}
}
