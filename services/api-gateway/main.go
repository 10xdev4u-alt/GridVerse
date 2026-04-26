package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
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

type Aggregated struct {
	EdgeID         string  `json:"edge_id"`
	Timestamp      string  `json:"timestamp"`
	MeterCount     int     `json:"meter_count"`
	AvgConsumption float64 `json:"avg_consumption"`
	AvgProduction  float64 `json:"avg_production"`
	PeakDetected   bool    `json:"peak_detected"`
	AnomalyCount   int     `json:"anomaly_count"`
}

type Forecast struct {
	Timestamp       string  `json:"timestamp"`
	PredictedDemand float64 `json:"predicted_demand"`
	LowerBound      float64 `json:"lower_bound"`
	UpperBound      float64 `json:"upper_bound"`
}

type StatusResponse struct {
	NATS   bool   `json:"nats"`
	DB     bool   `json:"db"`
	Uptime string `json:"uptime"`
}

var (
	db        *sql.DB
	nc        *nats.Conn
	upgrader  = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	clients   sync.Map
	startTime time.Time
)

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	port := getEnv("PORT", "8080")
	natsURL := getEnv("NATS_URL", "nats://nats:4222")
	dbURL := getEnv("DB_URL", "postgres://smartgrid:smartgrid@timescaledb:5432/smartgrid?sslmode=disable")
	startTime = time.Now()

	var err error
	nc, err = nats.Connect(natsURL,
		nats.ReconnectWait(2*time.Second),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()
	log.Printf("Connected to NATS at %s", natsURL)

	db, err = sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	if err := db.Ping(); err != nil {
		log.Printf("WARNING: DB not reachable: %v", err)
	} else {
		log.Println("Connected to TimescaleDB")
	}

	setupNATSSubscriptions()
	setupRoutes()

	log.Printf("API Gateway listening on :%s", port)
	go func() {
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("Shutting down API Gateway...")
}

func setupNATSSubscriptions() {
	nc.Subscribe("meter.>.readings", func(msg *nats.Msg) {
		var r Reading
		if err := json.Unmarshal(msg.Data, &r); err != nil {
			return
		}
		// Persist to DB
		if db != nil {
			_, err := db.Exec(`INSERT INTO meter_readings (time, meter_id, profile, consumption_kw, production_kw, voltage, status)
				VALUES ($1, $2, $3, $4, $5, $6, $7)`, r.Timestamp, r.MeterID, r.Profile, r.ConsumptionKW, r.ProductionKW, r.Voltage, r.Status)
			if err != nil {
				log.Printf("DB insert error: %v", err)
			}
		}
		// Broadcast to WebSocket clients
		broadcast(msg.Data)
	})

	nc.Subscribe("edge.aggregated", func(msg *nats.Msg) {
		var a Aggregated
		if err := json.Unmarshal(msg.Data, &a); err != nil {
			return
		}
		if db != nil {
			db.Exec(`INSERT INTO edge_aggregated (time, edge_id, meter_count, avg_consumption, avg_production, peak_detected, anomaly_count)
				VALUES ($1, $2, $3, $4, $5, $6, $7)`, a.Timestamp, a.EdgeID, a.MeterCount, a.AvgConsumption, a.AvgProduction, a.PeakDetected, a.AnomalyCount)
		}
		broadcast(msg.Data)
	})

	nc.Subscribe("forecast.demand", func(msg *nats.Msg) {
		var f Forecast
		if err := json.Unmarshal(msg.Data, &f); err != nil {
			return
		}
		if db != nil {
			db.Exec(`INSERT INTO forecasts (time, predicted_demand, lower_bound, upper_bound)
				VALUES ($1, $2, $3, $4)`, f.Timestamp, f.PredictedDemand, f.LowerBound, f.UpperBound)
		}
		broadcast(msg.Data)
	})
}

func broadcast(data []byte) {
	clients.Range(func(key, value interface{}) bool {
		conn := value.(*websocket.Conn)
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			clients.Delete(key)
			conn.Close()
		}
		return true
	})
}

func setupRoutes() {
	http.HandleFunc("/ws", handleWebSocket)
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/api/status", handleStatus)
	http.HandleFunc("/api/meters", handleMeters)
	http.HandleFunc("/api/readings/recent", handleRecentReadings)
	http.HandleFunc("/api/forecasts", handleForecasts)
	http.HandleFunc("/api/trades", handleTrades)
	http.HandleFunc("/api/eva", handleEVA)
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	clients.Store(conn.RemoteAddr().String(), conn)
	log.Printf("WebSocket client connected: %s", conn.RemoteAddr())

	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				clients.Delete(conn.RemoteAddr().String())
				conn.Close()
				return
			}
		}
	}()
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	natsOK := nc.Status() == nats.CONNECTED
	dbOK := db != nil && db.Ping() == nil
	resp := StatusResponse{NATS: natsOK, DB: dbOK, Uptime: time.Since(startTime).Round(time.Second).String()}
	data, _ := json.Marshal(resp)
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func handleMeters(w http.ResponseWriter, r *http.Request) {
	if db == nil {
		http.Error(w, `{"error":"db not available"}`, 503)
		return
	}
	rows, err := db.Query(`SELECT DISTINCT ON (meter_id) meter_id, profile, consumption_kw, production_kw, voltage, status, time
		FROM meter_readings ORDER BY meter_id, time DESC`)
	if err != nil {
		http.Error(w, `{"error":"query failed"}`, 500)
		return
	}
	defer rows.Close()

	type MeterRow struct {
		MeterID       string  `json:"meter_id"`
		Profile       string  `json:"profile"`
		ConsumptionKW float64 `json:"consumption_kw"`
		ProductionKW  float64 `json:"production_kw"`
		Voltage       float64 `json:"voltage"`
		Status        string  `json:"status"`
		LastSeen      string  `json:"last_seen"`
	}
	meters := make([]MeterRow, 0)
	for rows.Next() {
		var m MeterRow
		var t time.Time
		rows.Scan(&m.MeterID, &m.Profile, &m.ConsumptionKW, &m.ProductionKW, &m.Voltage, &m.Status, &t)
		m.LastSeen = t.Format(time.RFC3339)
		meters = append(meters, m)
	}
	data, _ := json.Marshal(meters)
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func handleRecentReadings(w http.ResponseWriter, r *http.Request) {
	if db == nil {
		http.Error(w, `{"error":"db not available"}`, 503)
		return
	}
	rows, err := db.Query(`SELECT time, meter_id, profile, consumption_kw, production_kw, voltage, status
		FROM meter_readings WHERE time > NOW() - INTERVAL '5 minutes' ORDER BY time DESC LIMIT 500`)
	if err != nil {
		http.Error(w, `{"error":"query failed"}`, 500)
		return
	}
	defer rows.Close()
	writeJSONRows(w, rows)
}

func handleForecasts(w http.ResponseWriter, r *http.Request) {
	if db == nil {
		http.Error(w, `{"error":"db not available"}`, 503)
		return
	}
	rows, err := db.Query(`SELECT time, predicted_demand, lower_bound, upper_bound
		FROM forecasts ORDER BY time DESC LIMIT 50`)
	if err != nil {
		http.Error(w, `{"error":"query failed"}`, 500)
		return
	}
	defer rows.Close()
	writeJSONRows(w, rows)
}

func handleTrades(w http.ResponseWriter, r *http.Request) {
	if db == nil {
		http.Error(w, `{"error":"db not available"}`, 503)
		return
	}
	rows, err := db.Query(`SELECT time, event_type, buyer, seller, amount_kwh, price_per_kwh, tx_hash
		FROM trade_events ORDER BY time DESC LIMIT 100`)
	if err != nil {
		http.Error(w, `{"error":"query failed"}`, 500)
		return
	}
	defer rows.Close()
	writeJSONRows(w, rows)
}

func handleEVA(w http.ResponseWriter, r *http.Request) {
	if db == nil {
		http.Error(w, `{"error":"db not available"}`, 503)
		return
	}
	rows, err := db.Query(`SELECT id, task_name, pv, ev, ac, status, updated_at FROM eva_metrics ORDER BY id`)
	if err != nil {
		http.Error(w, `{"error":"query failed"}`, 500)
		return
	}
	defer rows.Close()
	writeJSONRows(w, rows)
}

func writeJSONRows(w http.ResponseWriter, rows *sql.Rows) {
	cols, _ := rows.Columns()
	results := make([]map[string]interface{}, 0)
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		rows.Scan(ptrs...)
		row := make(map[string]interface{})
		for i, c := range cols {
			if b, ok := vals[i].([]byte); ok {
				row[c] = string(b)
			} else if t, ok := vals[i].(time.Time); ok {
				row[c] = t.Format(time.RFC3339)
			} else {
				row[c] = vals[i]
			}
		}
		results = append(results, row)
	}
	data, _ := json.Marshal(results)
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}