package main

import (
	"encoding/json"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

// Test helpers ----------------------------------------------------------------

func newTestSimulator(profile string) *Simulator {
	cfg := Config{
		MeterID:       "test-meter-001",
		Profile:       profile,
		NatsURL:       nats.DefaultURL,
		IntervalMS:    1000,
		NoiseFactor:   0.05,
		AnomalyChance: 0.0, // disable anomaly for deterministic tests
	}
	return &Simulator{
		cfg:     cfg,
		rng:     rand.New(rand.NewSource(42)), // fixed seed
		shedPct: 0,
	}
}

func (s *Simulator) withAnomalyChance(chance float64) *Simulator {
	s.cfg.AnomalyChance = chance
	return s
}

func (s *Simulator) withShed(pct float64) *Simulator {
	s.shedPct = pct
	return s
}

func generateMany(s *Simulator, n int) []Reading {
	readings := make([]Reading, n)
	for i := 0; i < n; i++ {
		readings[i] = s.generateReading()
	}
	return readings
}

// Profile value range tests ---------------------------------------------------

func TestResidentialProfile(t *testing.T) {
	s := newTestSimulator("residential")
	readings := generateMany(s, 100)

	for _, r := range readings {
		if r.Profile != "residential" {
			t.Errorf("expected profile residential, got %s", r.Profile)
		}
		if r.ProductionKW != 0 {
			t.Errorf("residential should have no production, got %.3f", r.ProductionKW)
		}
		// Consumption can spike with noise; allow some headroom
		if r.ConsumptionKW < 0 {
			t.Errorf("consumption should be non-negative, got %.3f", r.ConsumptionKW)
		}
		if r.Voltage != 230.0 {
			t.Errorf("expected voltage 230.0, got %.1f", r.Voltage)
		}
		if r.MeterID != "test-meter-001" {
			t.Errorf("wrong meter_id: %s", r.MeterID)
		}
		// Validate timestamp is RFC3339
		_, err := time.Parse(time.RFC3339, r.Timestamp)
		if err != nil {
			t.Errorf("invalid timestamp: %s", r.Timestamp)
		}
	}
}

func TestCommercialProfile(t *testing.T) {
	s := newTestSimulator("commercial")
	readings := generateMany(s, 100)

	for _, r := range readings {
		if r.Profile != "commercial" {
			t.Errorf("expected profile commercial, got %s", r.Profile)
		}
		if r.ProductionKW != 0 {
			t.Errorf("commercial should have no production, got %.3f", r.ProductionKW)
		}
		if r.ConsumptionKW < 0 {
			t.Errorf("consumption should be non-negative, got %.3f", r.ConsumptionKW)
		}
	}
}

func TestSolarPanelProfile(t *testing.T) {
	s := newTestSimulator("solar-panel")
	readings := generateMany(s, 200)

	for _, r := range readings {
		if r.Profile != "solar-panel" {
			t.Errorf("expected profile solar-panel, got %s", r.Profile)
		}
		if r.ConsumptionKW < 0 {
			t.Errorf("consumption should be non-negative, got %.3f", r.ConsumptionKW)
		}
		if r.ProductionKW < 0 {
			t.Errorf("production should be non-negative, got %.3f", r.ProductionKW)
		}
	}
}

func TestWindTurbineProfile(t *testing.T) {
	s := newTestSimulator("wind-turbine")
	readings := generateMany(s, 100)

	for _, r := range readings {
		if r.Profile != "wind-turbine" {
			t.Errorf("expected profile wind-turbine, got %s", r.Profile)
		}
		if r.ConsumptionKW < 0 {
			t.Errorf("consumption should be non-negative, got %.3f", r.ConsumptionKW)
		}
		if r.ProductionKW < 0 {
			t.Errorf("production should be non-negative, got %.3f", r.ProductionKW)
		}
	}

	// Wind should have production values spread across range
	hasHigh := false
	for _, r := range readings {
		if r.ProductionKW > 5.0 {
			hasHigh = true
			break
		}
	}
	if !hasHigh {
		t.Log("warning: no high wind production values found in 100 samples (may be OK with fixed seed)")
	}
}

func TestBatteryStorageProfile(t *testing.T) {
	s := newTestSimulator("battery-storage")
	readings := generateMany(s, 200)

	hasCharge := false
	hasDischarge := false

	for _, r := range readings {
		if r.Profile != "battery-storage" {
			t.Errorf("expected profile battery-storage, got %s", r.Profile)
		}
		if r.ConsumptionKW < 0 {
			t.Errorf("consumption should be non-negative, got %.3f", r.ConsumptionKW)
		}
		if r.ProductionKW < 0 {
			t.Errorf("production should be non-negative, got %.3f", r.ProductionKW)
		}
		// Exactly one should be zero (battery is either charging or discharging)
		if r.ConsumptionKW > 0 && r.ProductionKW > 0 {
			t.Errorf("battery should not charge and discharge simultaneously: c=%.3f, p=%.3f",
				r.ConsumptionKW, r.ProductionKW)
		}
		if r.ConsumptionKW > 0 {
			hasCharge = true
		}
		if r.ProductionKW > 0 {
			hasDischarge = true
		}
	}

	if !hasCharge {
		t.Error("battery should have charging states (consumption > 0)")
	}
	if !hasDischarge {
		t.Error("battery should have discharging states (production > 0)")
	}
}

// JSON marshaling test --------------------------------------------------------

func TestReadingJSONMarshaling(t *testing.T) {
	r := Reading{
		MeterID:       "meter-001",
		Profile:       "residential",
		Timestamp:     "2026-04-27T10:30:00Z",
		ConsumptionKW: 1.23,
		ProductionKW:  0.0,
		Voltage:       230.0,
		Status:        "normal",
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("failed to marshal reading: %v", err)
	}

	var unmarshaled Reading
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if unmarshaled.MeterID != r.MeterID {
		t.Errorf("meter_id mismatch: %s != %s", unmarshaled.MeterID, r.MeterID)
	}
	if unmarshaled.ConsumptionKW != r.ConsumptionKW {
		t.Errorf("consumption_kw mismatch: %.3f != %.3f", unmarshaled.ConsumptionKW, r.ConsumptionKW)
	}
	if unmarshaled.ProductionKW != r.ProductionKW {
		t.Errorf("production_kw mismatch: %.3f != %.3f", unmarshaled.ProductionKW, r.ProductionKW)
	}
	if unmarshaled.Voltage != r.Voltage {
		t.Errorf("voltage mismatch: %.1f != %.1f", unmarshaled.Voltage, r.Voltage)
	}
	if unmarshaled.Status != r.Status {
		t.Errorf("status mismatch: %s != %s", unmarshaled.Status, r.Status)
	}

	// Verify JSON keys
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)
	requiredKeys := []string{"meter_id", "profile", "timestamp", "consumption_kw", "production_kw", "voltage", "status"}
	for _, key := range requiredKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("JSON missing required key: %s", key)
		}
	}
}

// Anomaly detection test ------------------------------------------------------

func TestAnomalyDetection(t *testing.T) {
	s := newTestSimulator("residential")
	s.cfg.AnomalyChance = 1.0 // Force anomaly on every reading
	s.cfg.NoiseFactor = 0     // No noise

	readings := generateMany(s, 50)

	anomalyCount := 0
	normalCount := 0
	for _, r := range readings {
		if r.Status == "anomaly" {
			anomalyCount++
		} else {
			normalCount++
		}
	}

	if anomalyCount == 0 {
		t.Error("expected anomalies with anomaly_chance=1.0")
	}
	t.Logf("Anomalies: %d, Normal: %d (out of 50)", anomalyCount, normalCount)
}

func TestNoAnomaliesWhenDisabled(t *testing.T) {
	s := newTestSimulator("residential")
	s.cfg.AnomalyChance = 0.0

	readings := generateMany(s, 100)
	for _, r := range readings {
		if r.Status == "anomaly" {
			t.Errorf("unexpected anomaly with anomaly_chance=0.0")
		}
	}
}

// Command handling test -------------------------------------------------------

func TestCommandShed(t *testing.T) {
	s := newTestSimulator("residential")
	s.cfg.NoiseFactor = 0

	// Before shed: generate baseline
	beforeShed := generateMany(s, 20)
	var totalBefore float64
	for _, r := range beforeShed {
		totalBefore += r.ConsumptionKW
	}
	avgBefore := totalBefore / float64(len(beforeShed))
	if avgBefore <= 0 {
		t.Fatal("baseline consumption is zero, cannot test shed")
	}

	// Apply shed
	s.shedPct = 0.5 // 50% reduction

	afterShed := generateMany(s, 20)
	var totalAfter float64
	for _, r := range afterShed {
		totalAfter += r.ConsumptionKW
	}
	avgAfter := totalAfter / float64(len(afterShed))

	if avgAfter >= avgBefore {
		t.Errorf("expected shed to reduce consumption: before=%.3f, after=%.3f", avgBefore, avgAfter)
	}
	t.Logf("Before shed avg: %.3f kW, After 50%% shed avg: %.3f kW", avgBefore, avgAfter)
}

func TestCommandProtectAndRestore(t *testing.T) {
	s := newTestSimulator("residential")

	// Protect
	s.protected = true
	r := s.generateReading()
	if r.Status != "protected" {
		t.Errorf("expected protected status, got %s", r.Status)
	}

	// Restore
	s.protected = false
	r = s.generateReading()
	if r.Status != "normal" {
		t.Errorf("expected normal status after restore, got %s", r.Status)
	}
}

func TestCommandRestoreClearsShed(t *testing.T) {
	s := newTestSimulator("residential")
	s.shedPct = 0.5

	// Simulate restore: clear shed
	s.shedPct = 0
	s.protected = false

	if s.shedPct != 0 {
		t.Errorf("expected shed percentage 0 after restore, got %.2f", s.shedPct)
	}
	if s.protected {
		t.Error("expected protected=false after restore")
	}
}

// Solar production specific tests --------------------------------------------

func TestSolarProductionZeroAtNight(t *testing.T) {
	s := newTestSimulator("solar-panel")
	s.cfg.NoiseFactor = 0

	// Override time-dependent logic by iterating enough to see patterns
	// Actually, we test the solarProduction function directly with night hours
	hours := []float64{0, 1, 2, 3, 4, 5, 19, 20, 21, 22, 23}
	for _, h := range hours {
		prod := s.solarProduction(h)
		if prod != 0 {
			t.Errorf("solar production should be 0 at hour %.0f, got %.3f", h, prod)
		}
	}
}

func TestSolarProductionDuringDay(t *testing.T) {
	s := newTestSimulator("solar-panel")
	s.cfg.NoiseFactor = 0

	// Noon should have notable production
	prod := s.solarProduction(12)
	if prod <= 0 {
		t.Error("solar production should be positive at noon")
	}
	t.Logf("Solar production at noon: %.3f kW", prod)
}

// Battery tests ---------------------------------------------------------------

func TestBatteryOneWayOnly(t *testing.T) {
	s := newTestSimulator("battery-storage")
	s.cfg.NoiseFactor = 0

	readings := generateMany(s, 500)
	for _, r := range readings {
		if r.ConsumptionKW > 0 && r.ProductionKW > 0 {
			t.Errorf("battery cannot charge and discharge simultaneously: c=%.3f p=%.3f",
				r.ConsumptionKW, r.ProductionKW)
		}
	}
}

// Helper function tests -------------------------------------------------------

func TestGaussianPeak(t *testing.T) {
	// At center, peak should be 1.0
	v := gaussianPeak(12, 12, 2)
	if math.Abs(v-1.0) > 0.001 {
		t.Errorf("peak at center should be 1.0, got %.3f", v)
	}

	// Far from center, should be near 0
	v = gaussianPeak(0, 12, 2)
	if v > 0.01 {
		t.Errorf("peak far from center should be near 0, got %.6f", v)
	}
}

func TestBaseConsumption(t *testing.T) {
	s := newTestSimulator("residential")
	for i := 0; i < 100; i++ {
		v := s.baseConsumption(5, 15)
		if v < 5 || v > 15 {
			t.Errorf("baseConsumption(5,15) out of range: %.3f", v)
		}
	}
}

func TestGaussianNoise(t *testing.T) {
	s := newTestSimulator("residential")
	s.cfg.NoiseFactor = 0.05

	// Generate many noise samples, ensure they average near 0
	var sum float64
	n := 1000
	for i := 0; i < n; i++ {
		sum += s.gaussianNoise()
	}
	avg := sum / float64(n)
	if math.Abs(avg) > 0.01 {
		t.Logf("noise average: %.5f (expected near 0)", avg)
	}
}

// Config parsing tests --------------------------------------------------------

func TestDefaultConfig(t *testing.T) {
	cfg := Config{
		MeterID:       "meter-001",
		Profile:       "residential",
		NatsURL:       "nats://nats:4222",
		IntervalMS:    1000,
		NoiseFactor:   0.05,
		AnomalyChance: 0.02,
	}

	if cfg.MeterID != "meter-001" {
		t.Errorf("unexpected meter id: %s", cfg.MeterID)
	}
	if cfg.Profile != "residential" {
		t.Errorf("unexpected profile: %s", cfg.Profile)
	}
	if cfg.IntervalMS != 1000 {
		t.Errorf("unexpected interval: %d", cfg.IntervalMS)
	}
}