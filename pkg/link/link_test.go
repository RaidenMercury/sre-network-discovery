package link

import (
	"testing"
)

func TestIsBlacklisted(t *testing.T) {
	cases := []struct {
		name     string
		expected bool
	}{
		{"Ethernet0", false},
		{"eth0", false},
		{"enp3s0", false},
		{"vEthernet (Default Switch)", true},
		{"docker0", true},
		{"tailscale0", true},
		{"OpenVPN TAP Adapter", true},
		{"Hyper-V Virtual Adapter", true},
	}

	for _, c := range cases {
		got := IsBlacklisted(c.name)
		if got != c.expected {
			t.Errorf("IsBlacklisted(%q) = %v, expected %v", c.name, got, c.expected)
		}
	}
}

func TestGetEligibleAdapters(t *testing.T) {
	adapters, err := GetEligibleAdapters()
	if err != nil {
		t.Fatalf("unexpected error getting adapters: %v", err)
	}
	for _, a := range adapters {
		if IsBlacklisted(a.Name) {
			t.Errorf("found blacklisted adapter in eligible list: %s", a.Name)
		}
	}
}

