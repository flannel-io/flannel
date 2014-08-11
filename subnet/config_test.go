package subnet

import (
	"testing"
)

func TestConfigDefaults(t *testing.T) {
	s := `{ "network": "10.3.0.0/16" }`

	cfg, err := ParseConfig(s)
	if err != nil {
		t.Fatalf("ParseConfig failed: %s", err)
	}

	expectedNet := "10.3.0.0/16"
	if cfg.Network.String() != expectedNet {
		t.Errorf("Network mismatch: expected %s, got %s", expectedNet, cfg.Network)
	}

	if cfg.FirstIP.String() != "10.3.0.0" {
		t.Errorf("FirstIP mismatch, expected 10.3.0.0, got %s", cfg.FirstIP)
	}

	if cfg.LastIP.String() != "10.3.255.0" {
		t.Errorf("LastIP mismatch, expected 10.3.255.0, got %s", cfg.LastIP)
	}

	if cfg.HostSubnet != 24 {
		t.Errorf("HostSubnet mismatch: expected 24, got %d", cfg.HostSubnet)
	}
}

func TestConfigOverrides(t *testing.T) {
	s := `{ "Network": "10.3.0.0/16", "FirstIP": "10.3.5.0", "LastIP": "10.3.8.0", "HostSubnet": 28 }`

	cfg, err := ParseConfig(s)
	if err != nil {
		t.Fatalf("ParseConfig failed: %s", err)
	}

	expectedNet := "10.3.0.0/16"
	if cfg.Network.String() != expectedNet {
		t.Errorf("Network mismatch: expected %s, got %s", expectedNet, cfg.Network)
	}

	if cfg.FirstIP.String() != "10.3.5.0" {
		t.Errorf("FirstIP mismatch: expected 10.3.5.0, got %s", cfg.FirstIP)
	}

	if cfg.LastIP.String() != "10.3.8.0" {
		t.Errorf("LastIP mismatch: expected 10.3.8.0, got %s", cfg.LastIP)
	}

	if cfg.HostSubnet != 28 {
		t.Errorf("HostSubnet mismatch: expected 28, got %d", cfg.HostSubnet)
	}
}
