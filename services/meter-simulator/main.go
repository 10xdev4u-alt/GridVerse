package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
)

// Reading represents a single smart meter reading published to NATS.
type Reading struct {
	MeterID       string  `json:"meter_id"`
	Profile       string  `json:"profile"`
	Timestamp     string  `json:"timestamp"`
	ConsumptionKW float64 `json:"consumption_kw"`
	ProductionKW  float64 `json:"production_kw"`
	Voltage       float64 `json:"voltage"`
	Status        string  `json:"status"`
}

// Config holds all configuration parsed from environment variables.
type Config struct {
	MeterID      string
	Profile      string
	NatsURL      string
	IntervalMS   int
	NoiseFactor  float64
	AnomalyChance float64
}

func loadConfig() Config {
	return Config{
		MeterID:       envStr("METER_ID", "meter-001"),
		Profile:       envStr("PROFILE", "residential"),
		NatsURL:       envStr("NATS_URL", "nats://nats:4222"),
		IntervalMS:    envInt("INTERVAL_MS", 1000),
		NoiseFactor:   envFloat("NOISE_FACTOR", 0.05),
		AnomalyChance: envFloat("ANOMALY_CHANCE", 0.02),
	}
}

func envStr(key, defaultVal string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return defaultVal
}

func envInt(key string, defaultVal int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}

func envFloat(key string, defaultVal float64) float64 {
	if v, ok := os.LookupEnv(key); ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return defaultVal
}

// Simulator manages the lifecycle and state of a single meter.
type Simulator struct {
	cfg       Config
	nc        *nats.Conn
	rng       *rand.Rand
	shedPct   float64
	protected bool
}

func NewSimulator(cfg Config, nc *nats.Conn) *Simulator {
	return &Simulator{
		cfg:     cfg,
		nc:      nc,
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
		shedPct: 0,
	}
}

// gaussianNoise returns a Gaussian-distributed value with mean 0 and stddev 1.
func (s *Simulator) gaussianNoise() float64 {
	return s.rng.NormFloat64() * s.cfg.NoiseFactor
}

// generateReading produces a Reading based on the current profile, time of day, and state.
func (s *Simulator) generateReading() Reading {
	now := time.Now().UTC()
	hour := float64(now.Hour()) + float64(now.Minute())/60.0

	var consumptionKW, productionKW float64

	switch s.cfg.Profile {
	case "residential":
		consumptionKW = s.residentialConsumption(hour)
		productionKW = 0
	case "commercial":
		consumptionKW = s.commercialConsumption(hour)
		productionKW = 0
	case "solar-panel":
		consumptionKW = s.baseConsumption(0.1, 0.5)
		productionKW = s.solarProduction(hour)
	case "wind-turbine":
		consumptionKW = s.baseConsumption(0.1, 0.1)
		productionKW = s.windProduction()
	case "battery-storage":
		consumptionKW, productionKW = s.batteryPower()
	default:
		consumptionKW = s.residentialConsumption(hour)
		productionKW = 0
	}

	// Apply shedding percentage to consumption only
	if s.shedPct > 0 {
		consumptionKW *= (1 - s.shedPct)
	}

	// Add Gaussian noise (only when there is real load/generation)
	if consumptionKW > 0.01 {
		consumptionKW += s.gaussianNoise() * consumptionKW
	}
	if productionKW > 0.01 {
		productionKW += s.gaussianNoise() * productionKW
	}

	// Clamp to non-negative
	consumptionKW = math.Max(0, consumptionKW)
	productionKW = math.Max(0, productionKW)

	// Inject anomaly
	status := "normal"
	if s.rng.Float64() < s.cfg.AnomalyChance {
		spike := 3.0 + s.rng.Float64()*2.0 // 3x to 5x
		if productionKW > 0 {
			productionKW *= spike
		}
		if consumptionKW > 0 {
			consumptionKW *= spike
		}
		status = "anomaly"
	}

	if s.protected {
		status = "protected"
	}

	// Round for clean output
	consumptionKW = math.Round(consumptionKW*1000) / 1000
	productionKW = math.Round(productionKW*1000) / 1000

	return Reading{
		MeterID:       s.cfg.MeterID,
		Profile:       s.cfg.Profile,
		Timestamp:     now.Format(time.RFC3339),
		ConsumptionKW: consumptionKW,
		ProductionKW:  productionKW,
		Voltage:       230.0,
		Status:        status,
	}
}

func (s *Simulator) residentialConsumption(hour float64) float64 {
	base := s.baseConsumption(0.5, 2.0)
	// Morning peak 6-9am, evening peak 5-9pm
	morningPeak := gaussianPeak(hour, 7.5, 1.5)
	eveningPeak := gaussianPeak(hour, 19.0, 2.0)
	multiplier := 1.0 + morningPeak*0.5 + eveningPeak*0.7
	return base * multiplier
}

func (s *Simulator) commercialConsumption(hour float64) float64 {
	base := s.baseConsumption(5, 15)
	// Peak during business hours 9am-5pm
	if hour >= 9 && hour < 17 {
		return base * (1.0 + s.rng.Float64()*0.3)
	}
	return base * 0.4 // off-hours
}

func (s *Simulator) solarProduction(hour float64) float64 {
	// Sinusoidal production 6am-6pm, peak at noon, zero at night
	if hour < 6 || hour > 18 {
		return 0
	}
	// Map 6-18 to 0-pi
	frac := (hour - 6) / 12.0
	sinVal := math.Sin(frac * math.Pi)
	peak := 2.0 + s.rng.Float64()*6.0 // 2-8 kW peak
	return peak * sinVal
}

func (s *Simulator) windProduction() float64 {
	// High variability wind
	base := 1.0 + s.rng.Float64()*9.0 // 1-10 kW
	variability := 1.0 + s.rng.Float64()*2.0
	return base * variability
}

func (s *Simulator) batteryPower() (float64, float64) {
	power := s.rng.Float64() * 5.0 // 0-5 kW
	if s.rng.Float64() < 0.5 {
		// Discharging: production
		return 0, power
	}
	// Charging: consumption
	return power, 0
}

func (s *Simulator) baseConsumption(min, max float64) float64 {
	return min + s.rng.Float64()*(max-min)
}

// gaussianPeak returns a Gaussian-shaped peak centered at `center` hour with given stddev.
func gaussianPeak(hour, center, stddev float64) float64 {
	diff := hour - center
	return math.Exp(-(diff * diff) / (2 * stddev * stddev))
}

func (s *Simulator) handleCommand(m *nats.Msg) {
	cmd := string(m.Data)
	log.Printf("[%s] Received command: %s", s.cfg.MeterID, cmd)

	switch cmd {
	case "shed":
		s.shedPct = 0.3 // reduce by 30%
		log.Printf("[%s] Load shedding activated (30%%)", s.cfg.MeterID)
	case "protect":
		s.protected = true
		s.shedPct = 0
		log.Printf("[%s] Protected mode activated", s.cfg.MeterID)
	case "restore":
		s.shedPct = 0
		s.protected = false
		log.Printf("[%s] Restored to normal operation", s.cfg.MeterID)
	default:
		// Try to parse as shed percentage like "shed:50"
		if len(cmd) > 5 && cmd[:5] == "shed:" {
			if pct, err := strconv.ParseFloat(cmd[5:], 64); err == nil && pct >= 0 && pct <= 100 {
				s.shedPct = pct / 100.0
				s.protected = false
				log.Printf("[%s] Load shedding at %.0f%%", s.cfg.MeterID, pct)
			}
		}
	}
}

func (s *Simulator) run() error {
	// Connect to NATS
	var err error
	s.nc, err = nats.Connect(s.cfg.NatsURL,
		nats.Name(s.cfg.MeterID),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			log.Printf("[%s] Disconnected from NATS: %v", s.cfg.MeterID, err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Printf("[%s] Reconnected to NATS at %v", s.cfg.MeterID, nc.ConnectedUrl())
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			log.Printf("[%s] NATS connection closed: %v", s.cfg.MeterID, nc.LastError())
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}
	log.Printf("[%s] Connected to NATS at %s", s.cfg.MeterID, s.cfg.NatsURL)
	log.Printf("[%s] Profile: %s | Interval: %dms | Noise: %.2f | Anomaly chance: %.2f%%",
		s.cfg.MeterID, s.cfg.Profile, s.cfg.IntervalMS, s.cfg.NoiseFactor, s.cfg.AnomalyChance*100)

	// Subscribe to commands
	cmdSubj := fmt.Sprintf("meter.%s.commands", s.cfg.MeterID)
	_, err = s.nc.Subscribe(cmdSubj, s.handleCommand)
	if err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", cmdSubj, err)
	}
	log.Printf("[%s] Listening for commands on %s", s.cfg.MeterID, cmdSubj)

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Reading publish ticker
	readingSubj := fmt.Sprintf("meter.%s.readings", s.cfg.MeterID)
	ticker := time.NewTicker(time.Duration(s.cfg.IntervalMS) * time.Millisecond)
	defer ticker.Stop()

	log.Printf("[%s] Publishing readings to %s", s.cfg.MeterID, readingSubj)

	for {
		select {
		case <-sigCh:
			log.Printf("[%s] Shutting down gracefully...", s.cfg.MeterID)
			return s.nc.Drain()

		case <-ticker.C:
			reading := s.generateReading()
			data, err := json.Marshal(reading)
			if err != nil {
				log.Printf("[%s] Failed to marshal reading: %v", s.cfg.MeterID, err)
				continue
			}
			if err := s.nc.Publish(readingSubj, data); err != nil {
				log.Printf("[%s] Failed to publish reading: %v", s.cfg.MeterID, err)
				continue
			}
			if reading.Status == "anomaly" {
				log.Printf("[%s] ANOMALY: consumption=%.3f kW, production=%.3f kW",
					s.cfg.MeterID, reading.ConsumptionKW, reading.ProductionKW)
			}
		}
	}
}

func main() {
	cfg := loadConfig()
	sim := NewSimulator(cfg, nil) // Use the constructor to initialize RNG

	if err := sim.run(); err != nil {
		log.Fatalf("[%s] Fatal error: %v", cfg.MeterID, err)
	}
	log.Printf("[%s] Shutdown complete", cfg.MeterID)
}