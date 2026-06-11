package config

import (
	"errors"
	"strings"
	"testing"
)

// TestUsageScanBudgetDefault: an absent usageScanBudget resolves to the 20 000 default
// (017 US1/FR-006, research D2).
func TestUsageScanBudgetDefault(t *testing.T) {
	c, err := Load(writeConfig(t, validYAML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.UsageScanBudget != nil {
		t.Errorf("UsageScanBudget = %v, want nil for an absent key", *c.UsageScanBudget)
	}
	if got := c.ResolvedUsageScanBudget(); got != DefaultUsageScanBudget {
		t.Errorf("ResolvedUsageScanBudget() = %d, want %d", got, DefaultUsageScanBudget)
	}
}

// TestUsageScanBudgetZeroDisables: an explicit 0 is valid and means "ambient scanning
// off" — distinct from absent (017 FR-006).
func TestUsageScanBudgetZeroDisables(t *testing.T) {
	body := strings.Replace(validYAML, "current-context: local",
		"current-context: local\nusageScanBudget: 0", 1)
	c, err := Load(writeConfig(t, body))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.UsageScanBudget == nil || *c.UsageScanBudget != 0 {
		t.Fatalf("UsageScanBudget = %v, want explicit 0", c.UsageScanBudget)
	}
	if got := c.ResolvedUsageScanBudget(); got != 0 {
		t.Errorf("ResolvedUsageScanBudget() = %d, want 0 (ambient off)", got)
	}
}

// TestUsageScanBudgetCustom: a positive value passes through.
func TestUsageScanBudgetCustom(t *testing.T) {
	body := strings.Replace(validYAML, "current-context: local",
		"current-context: local\nusageScanBudget: 5000", 1)
	c, err := Load(writeConfig(t, body))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := c.ResolvedUsageScanBudget(); got != 5000 {
		t.Errorf("ResolvedUsageScanBudget() = %d, want 5000", got)
	}
}

// TestUsageScanBudgetNegativeInvalid: a negative budget fails validation (017 D2).
func TestUsageScanBudgetNegativeInvalid(t *testing.T) {
	body := strings.Replace(validYAML, "current-context: local",
		"current-context: local\nusageScanBudget: -1", 1)
	_, err := Load(writeConfig(t, body))
	if !errors.Is(err, ErrInvalid) {
		t.Errorf("Load(negative budget) = %v, want ErrInvalid", err)
	}
}
