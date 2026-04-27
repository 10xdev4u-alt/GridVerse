package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/lib/pq"
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

type TradeEvent struct {
	EventType   string  `json:"event_type"`
	Buyer       string  `json:"buyer"`
	Seller      string  `json:"seller"`
	AmountKWH   float64 `json:"amount_kwh"`
	PricePerKWH float64 `json:"price_per_kwh"`
	TxHash      string  `json:"tx_hash"`
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" { return v }
	return def
}

func main() {
	natsURL := getEnv("NATS_URL", "nats://nats:4222")
	dbURL := getEnv("DB_URL", "postgres://smartgrid:smartgrid@timescaledb:5432/smartgrid?sslmode=disable")

	nc, err := nats.Connect(natsURL,
		nats.ReconnectWait(2*time.Second),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()
	log.Printf("Blockchain Bridge connected to NATS at %s", natsURL)

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Printf("WARNING: DB not reachable: %v", err)
	} else {
		defer db.Close()
		log.Println("Connected to TimescaleDB")
	}

	// Track meter balances (kWh surplus/deficit)
	meterBalances := make(map[string]float64)
	var mu sync.Mutex

	// Subscribe to meter readings to detect surplus/deficit
	nc.Subscribe("meter.*.readings", func(msg *nats.Msg) {
		var r Reading
		if err := json.Unmarshal(msg.Data, &r); err != nil { return }

		net := r.ProductionKW - r.ConsumptionKW

		mu.Lock()
		meterBalances[r.MeterID] += net * (1.0 / 3600.0) // Convert kW to kWh per second
		balance := meterBalances[r.MeterID]
		mu.Unlock()

		// If significant surplus (>1 kWh), simulate a trade listing
		if balance > 1.0 {
			mu.Lock()
			meterBalances[r.MeterID] -= 1.0
			mu.Unlock()

			event := TradeEvent{
				EventType:   "EnergyListed",
				Seller:      r.MeterID,
				AmountKWH:   1.0,
				PricePerKWH: 0.12,
				TxHash:      fmt.Sprintf("0x%x", time.Now().UnixNano()),
			}
			publishTrade(nc, db, event)
		}

		// If significant deficit (< -1 kWh), simulate a purchase
		if balance < -1.0 {
			mu.Lock()
			meterBalances[r.MeterID] += 1.0
			mu.Unlock()

			// Find a random seller
			mu.Lock()
			var seller string
			for id, bal := range meterBalances {
				if bal > 1.0 && id != r.MeterID {
					seller = id
					meterBalances[id] -= 1.0
					break
				}
			}
			mu.Unlock()

			if seller != "" {
				event := TradeEvent{
					EventType:   "EnergyPurchased",
					Buyer:       r.MeterID,
					Seller:      seller,
					AmountKWH:   1.0,
					PricePerKWH: 0.12,
					TxHash:      fmt.Sprintf("0x%x", time.Now().UnixNano()),
				}
				publishTrade(nc, db, event)

				settlement := TradeEvent{
					EventType:   "SettlementCompleted",
					Buyer:       r.MeterID,
					Seller:      seller,
					AmountKWH:   1.0,
					PricePerKWH: 0.12,
					TxHash:      fmt.Sprintf("0x%x", time.Now().UnixNano()),
				}
				publishTrade(nc, db, settlement)
			}
		}
	})

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("Shutting down Blockchain Bridge...")
}

func publishTrade(nc *nats.Conn, db *sql.DB, event TradeEvent) {
	data, _ := json.Marshal(event)
	nc.Publish("trades.events", data)
	log.Printf("Trade: %s | %s->%s | %.2f kWh @ $%.2f", event.EventType, event.Seller, event.Buyer, event.AmountKWH, event.PricePerKWH)

	if db != nil {
		db.Exec(`INSERT INTO trade_events (event_type, buyer, seller, amount_kwh, price_per_kwh, tx_hash)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			event.EventType, event.Buyer, event.Seller, event.AmountKWH, event.PricePerKWH, event.TxHash)
	}
}