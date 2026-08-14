package common

import (
	"net"
	"testing"
)

func TestSSRFProtectionRejectsPrivateAddresses(t *testing.T) {
	protection := &SSRFProtection{AllowedPorts: []int{80, 443}, ApplyIPFilterForDomain: true}
	for _, target := range []string{"http://127.0.0.1", "http://10.0.0.1", "http://[::1]"} {
		if err := protection.ValidateURL(target); err == nil {
			t.Fatalf("ValidateURL(%q) unexpectedly allowed private address", target)
		}
	}
}

func TestSSRFProtectionRejectsMalformedPorts(t *testing.T) {
	protection := &SSRFProtection{AllowedPorts: []int{80, 443}}
	for _, target := range []string{"http://example.com:bad", "http://example.com:9999"} {
		if err := protection.ValidateURL(target); err == nil {
			t.Fatalf("ValidateURL(%q) unexpectedly allowed malformed/disallowed port", target)
		}
	}
}

func TestIsPrivateIPIncludesUnspecifiedAndMappedAddresses(t *testing.T) {
	for _, raw := range []string{"0.0.0.0", "127.0.0.1", "192.168.1.1", "::1", "::ffff:127.0.0.1"} {
		if !IsPrivateIP(net.ParseIP(raw)) {
			t.Fatalf("IsPrivateIP(%q) = false", raw)
		}
	}
}
