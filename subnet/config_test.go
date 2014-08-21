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

	if cfg.SubnetMin.String() != "10.3.1.0" {
		t.Errorf("SubnetMin mismatch, expected 10.3.1.0, got %s", cfg.SubnetMin)
	}

	if cfg.SubnetMax.String() != "10.3.255.0" {
		t.Errorf("SubnetMax mismatch, expected 10.3.255.0, got %s", cfg.SubnetMax)
	}

	if cfg.SubnetLen != 24 {
		t.Errorf("SubnetLen mismatch: expected 24, got %d", cfg.SubnetLen)
	}
}

func TestConfigOverrides(t *testing.T) {
	s := `{ "Network": "10.3.0.0/16", "SubnetMin": "10.3.5.0", "SubnetMax": "10.3.8.0", "SubnetLen": 28 }`

	cfg, err := ParseConfig(s)
	if err != nil {
		t.Fatalf("ParseConfig failed: %s", err)
	}

	expectedNet := "10.3.0.0/16"
	if cfg.Network.String() != expectedNet {
		t.Errorf("Network mismatch: expected %s, got %s", expectedNet, cfg.Network)
	}

	if cfg.SubnetMin.String() != "10.3.5.0" {
		t.Errorf("SubnetMin mismatch: expected 10.3.5.0, got %s", cfg.SubnetMin)
	}

	if cfg.SubnetMax.String() != "10.3.8.0" {
		t.Errorf("SubnetMax mismatch: expected 10.3.8.0, got %s", cfg.SubnetMax)
	}

	if cfg.SubnetLen != 28 {
		t.Errorf("SubnetLen mismatch: expected 28, got %d", cfg.SubnetLen)
	}
}
