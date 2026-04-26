package main

import (
	"encoding/json"
	"os"
	"testing"
)

func TestAggregatedJSON(t *testing.T) {
	agg := Aggregated{
		EdgeID:         "edge-001",
		Timestamp:      "2026-04-27T10:00:00Z",
		MeterCount:     5,
		AvgConsumption: 1.5,
		AvgProduction:  3.2,
		PeakDetected:   false,
		AnomalyCount:   0,
	}
	data, err := json.Marshal(agg)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}
	var result Aggregated
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
	if result.MeterCount != 5 {
		t.Errorf("Expected 5 meters, got %d", result.MeterCount)
	}
	if result.AvgConsumption != 1.5 {
		t.Errorf("Expected 1.5, got %f", result.AvgConsumption)
	}
}

func TestGetEnv(t *testing.T) {
	os.Setenv("TEST_VAR", "hello")
	if v := getEnv("TEST_VAR", "default"); v != "hello" {
		t.Errorf("Expected 'hello', got '%s'", v)
	}
	if v := getEnv("MISSING", "default"); v != "default" {
		t.Errorf("Expected 'default', got '%s'", v)
	}
}

func TestGetEnvFloat(t *testing.T) {
	os.Setenv("TEST_FLOAT", "3.14")
	if v := getEnvFloat("TEST_FLOAT", 1.0); v != 3.14 {
		t.Errorf("Expected 3.14, got %f", v)
	}
	if v := getEnvFloat("MISSING", 2.0); v != 2.0 {
		t.Errorf("Expected 2.0, got %f", v)
	}
}

func TestCommandJSON(t *testing.T) {
	cmd := Command{Command: "shed", Percent: 20, Duration: "30m"}
	data, _ := json.Marshal(cmd)
	var result Command
	json.Unmarshal(data, &result)
	if result.Command != "shed" {
		t.Errorf("Expected 'shed', got '%s'", result.Command)
	}
	if result.Percent != 20 {
		t.Errorf("Expected 20, got %d", result.Percent)
	}
}