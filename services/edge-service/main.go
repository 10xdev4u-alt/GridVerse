package main

import (
	"encoding/json"
	"log"
	"math"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
)

type Reading struct {
	MeterID       string  `json:"meter_id"`
	Profile       string  `json:"profile"`
	Timestamp     string  `json:"timestamp"`
	ConsumptionKW float64 `json:"consumption_kw"`
	ProductionKW  float64 `json:"production_kw"`
	Voltage       float64 `json:"voltage"`
	Status        string  `json:"status"`
}

type Aggregated struct {
	EdgeID         string  `json:"edge_id"`
	Timestamp      string  `json:"timestamp"`
	MeterCount     int     `json:"meter_count"`
	AvgConsumption float64 `json:"avg_consumption"`
	AvgProduction  float64 `json:"avg_production"`
	PeakDetected   bool    `json:"peak_detected"`
	AnomalyCount   int     `json:"anomaly_count"`
}

type Command struct {
	Command  string `json:"command"`
	Percent  int    `json:"percent,omitempty"`
	Duration string `json:"duration,omitempty"`
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func main() {
	edgeID := getEnv("EDGE_ID", "edge-001")
	natsURL := getEnv("NATS_URL", "nats://nats:4222")
	aggWindow := getEnvFloat("AGGREGATION_WINDOW", 5)
	peakThreshold := getEnvFloat("PEAK_THRESHOLD", 2.0)

	nc, err := nats.Connect(natsURL,
		nats.ReconnectWait(2*time.Second),
		nats.MaxReconnects(-1),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			log.Printf("NATS disconnected: %v", err)
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			log.Println("NATS reconnected")
		}),
	)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()
	log.Printf("Edge service %s connected to NATS at %s", edgeID, natsURL)

	var mu sync.Mutex
	readings := make([]Reading, 0)
	failedQueue := make([]Aggregated, 0)
	maxQueue := 1000

	// Subscribe to meter readings
	nc.Subscribe("meter.*.readings", func(msg *nats.Msg) {
		var r Reading
		if err := json.Unmarshal(msg.Data, &r); err != nil {
			log.Printf("Failed to unmarshal reading: %v", err)
			return
		}
		mu.Lock()
		readings = append(readings, r)
		mu.Unlock()
	})

	// Subscribe to commands (log and forward)
	nc.Subscribe("meter.*.commands", func(msg *nats.Msg) {
		var cmd Command
		if err := json.Unmarshal(msg.Data, &cmd); err != nil {
			log.Printf("Failed to unmarshal command: %v", err)
			return
		}
		log.Printf("Command on %s: %+v", msg.Subject, cmd)
	})

	// Aggregation ticker
	ticker := time.NewTicker(time.Duration(aggWindow) * time.Second)
	defer ticker.Stop()

	// Retry failed publishes
	retryTicker := time.NewTicker(10 * time.Second)
	defer retryTicker.Stop()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			mu.Lock()
			if len(readings) == 0 {
				mu.Unlock()
				continue
			}

			n := len(readings)
			var sumCons, sumProd float64
			anomalyCount := 0
			for _, r := range readings {
				sumCons += r.ConsumptionKW
				sumProd += r.ProductionKW
				if r.Status == "anomaly" {
					anomalyCount++
				}
			}
			avgCons := sumCons / float64(n)
			avgProd := sumProd / float64(n)

			// Peak detection
			peakDetected := false
			for _, r := range readings {
				if r.ConsumptionKW > peakThreshold*avgCons || r.ProductionKW > peakThreshold*avgProd {
					peakDetected = true
					break
				}
			}

			readings = readings[:0]
			mu.Unlock()

			agg := Aggregated{
				EdgeID:         edgeID,
				Timestamp:      time.Now().UTC().Format(time.RFC3339),
				MeterCount:     n,
				AvgConsumption: math.Round(avgCons*1000) / 1000,
				AvgProduction:  math.Round(avgProd*1000) / 1000,
				PeakDetected:   peakDetected,
				AnomalyCount:   anomalyCount,
			}

			data, _ := json.Marshal(agg)
			if err := nc.Publish("edge.aggregated", data); err != nil {
				log.Printf("Failed to publish aggregated data: %v", err)
				mu.Lock()
				if len(failedQueue) < maxQueue {
					failedQueue = append(failedQueue, agg)
				}
				mu.Unlock()
			}

		case <-retryTicker.C:
			mu.Lock()
			if len(failedQueue) > 0 {
				log.Printf("Retrying %d failed publishes", len(failedQueue))
				for _, agg := range failedQueue {
					data, _ := json.Marshal(agg)
					if err := nc.Publish("edge.aggregated", data); err == nil {
						failedQueue = failedQueue[1:]
					} else {
						break
					}
				}
			}
			mu.Unlock()

		case <-sig:
			log.Println("Shutting down edge service...")
			return
		}
	}
}