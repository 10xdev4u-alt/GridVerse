# IoT Smart Grid — Full System Design

**Date:** 2026-04-27
**Status:** Approved
**Type:** College Project — Demo Day

---

## 1. Overview

An IoT-based Smart Grid system that uses edge-computing and smart meters to balance renewable energy, secured by blockchain-based peer-to-peer energy trading, and visualized on a cloud dashboard. The system uses Exploratory Data Analysis (EDA) for demand forecasting, a custom DSL for load-shedding logic, and is managed via Work Breakdown Structure (WBS) and Earned Value Analysis (EVA).

**Key constraint:** This is a college demo project — all "IoT devices" are simulated via Docker containers running Go programs. No physical hardware required.

---

## 2. Architecture — Event-Driven Microkernel

```
┌─────────────────────────────────────────────────────────────┐
│                    CLOUD DASHBOARD (Next.js)                │
│          WebSocket │ REST API │ D3/Recharts Charts          │
└──────────────────────────┬──────────────────────────────────┘
                           │
                    ┌──────▼──────┐
                    │  API GATEWAY │  (Go — Gin/Fiber)
                    │  + WebSocket│
                    └──────┬──────┘
                           │
          ┌────────────────┼────────────────┐
          │                │                │
   ┌──────▼──────┐  ┌──────▼──────┐  ┌──────▼──────┐
   │  EDGE NODE  │  │  BLOCKCHAIN │  │  EDA ENGINE │
   │  SERVICE    │  │  SERVICE     │  │  (Python)   │
   │  (Go)       │  │  (Hardhat)   │  │             │
   └──────┬──────┘  └──────┬──────┘  └──────┬──────┘
          │                │                │
   ┌──────▼──────┐  ┌──────▼──────┐  ┌──────▼──────┐
   │  SMART METER│  │  ENERGY     │  │  FORECAST   │
   │  SIMULATORS │  │  TRADING    │  │  & EDA      │
   │  (Docker/Go)│  │  CONTRACTS  │  │  PIPELINE   │
   └─────────────┘  └─────────────┘  └─────────────┘
                           │
                    ┌──────▼──────┐
                    │  DSL ENGINE │
                    │  (Go — PEG) │
                    │  Load-Shed  │
                    └─────────────┘
```

**Message Bus:** NATS (lightweight, Go-native, supports pub/sub + request-reply)
**Time-Series DB:** TimescaleDB (PostgreSQL extension — SQL familiarity, built-in continuous aggregates)
**Relational DB:** PostgreSQL (users, contracts, WBS/EVA metadata)

---

## 3. Subsystems

### 3.1 Smart Meter Simulator Fleet (Docker/Go)

Each container simulates a smart meter producing readings at configurable intervals.

**Per-container behavior:**
- Generates power consumption/production readings (kWh)
- Supports profiles: residential, commercial, solar-panel, wind-turbine, battery-storage
- Publishes to NATS topic `meter.{meter_id}.readings`
- Receives DSL-executed load-shed commands on `meter.{meter_id}.commands`
- Configurable noise, seasonality, and anomaly injection (for EDA demo)

**Docker Compose orchestration:**
- Spin up 10–50+ meters via `docker compose up --scale meter=50`
- Each meter gets unique ID, profile, and config via environment variables
- Health checks and auto-restart

### 3.2 Edge Computing Service (Go)

Processes meter data close to the source before hitting the cloud.

**Responsibilities:**
- Subscribe to `meter.>.readings` (wildcard — all meters)
- Local aggregation: 5-second rolling averages, peak detection, anomaly flagging
- Forward aggregated data to cloud via NATS `edge.aggregated`
- Execute load-shed commands received from DSL engine
- Circuit-breaker pattern: if cloud is unreachable, queue locally and sync when reconnected

**Edge node simulation:** Multiple edge-node instances (one per "neighborhood" of meters), each aggregating a subset.

### 3.3 EDA & Demand Forecasting Engine (Python)

**Pipeline:**
1. **Ingest** — Pull raw + aggregated readings from TimescaleDB
2. **EDA** — Pandas-based profiling: distributions, correlations, seasonality decomposition, anomaly detection
3. **Feature Engineering** — Lag features, rolling statistics, time-of-day encoding
4. **Forecasting** — Lightweight model (Prophet or simple ARIMA) for 1-hour-ahead demand prediction
5. **Output** — Publish forecasts to NATS `forecast.demand`, store in TimescaleDB

**Demo-friendly:** Pre-built Jupyter notebook showing the EDA process step-by-step with matplotlib/seaborn plots. The live system runs the trained model as a Python service.

### 3.4 Custom DSL — Load-Shedding Logic (Go)

A domain-specific language for declaring load-shedding rules.

**Syntax example:**
```
rule "peak-shed-residential" {
  when: demand > 0.85 * capacity
        AND time.hour IN [17..21]
        AND meter.profile == "residential"
  action: shed(percent: 10, duration: 30m)
  priority: 2
}

rule "critical-protect" {
  when: meter.profile == "hospital"
        OR meter.profile == "datacenter"
  action: protect()
  priority: 0
}
```

**Engine:**
- PEG parser (using `peggo` or hand-rolled Go parser)
- Rules compiled to an AST, evaluated against real-time state
- Priority-based conflict resolution (lower number = higher priority)
- Actions publish to NATS `meter.{id}.commands`
- Hot-reload: rules file watched for changes, re-parsed without restart

### 3.5 Blockchain Energy Trading (Hardhat + Solidity)

**Smart Contracts:**

1. **EnergyToken.sol** — ERC-20 token representing 1 kWh. Minted when a meter reports surplus production.
2. **EnergyMarketplace.sol** — Peer-to-peer trading. Producers list energy at price, consumers buy. Order-book matching.
3. **Settlement.sol** — Automatic settlement after trade. Transfers tokens, logs transaction, emits events.

**Demo flow:**
1. Solar meter reports surplus → contract mints EnergyTokens
2. Meter owner lists tokens on marketplace at price
3. Consumer meter (deficit) buys tokens
4. Settlement contract executes transfer
5. Events emitted → dashboard updates in real-time

**Infrastructure:**
- Hardhat local node (Ganache-compatible)
- 10+ pre-funded test accounts (each mapped to a simulated meter)
- Deployment scripts + automated test suite

### 3.6 Cloud Dashboard (Next.js + WebSocket)

**Design philosophy:** Sleek, light, zero-restrictions. Apple/Stripe-tier fluidity.

**Layout:**
- **Left sidebar:** Navigation (Overview, Meters, Trading, Forecasts, DSL Rules, WBS/EVA)
- **Main area:** Content panels with smooth transitions
- **Top bar:** System status indicators (connected meters, grid load, blockchain sync)

**Pages:**

1. **Overview** — Real-time grid map (SVG/Canvas), total generation vs consumption, grid health score
2. **Meters** — Live table of all meters with sparklines, filterable by profile/status
3. **Trading** — Energy marketplace UI: order book, recent trades, price chart, wallet balances
4. **Forecasts** — Demand forecast chart (actual vs predicted), EDA insights panel, anomaly timeline
5. **DSL Rules** — Code editor (Monaco) for writing load-shed rules, syntax highlighting, live test button
6. **WBS/EVA** — Project management: task breakdown, earned value charts (CPI/SPI), budget tracking

**Tech:**
- Next.js 14 (App Router)
- Tailwind CSS + custom design tokens (CSS variables)
- Recharts for time-series, D3 for grid map
- WebSocket via `ws` or Socket.IO for real-time updates
- Framer Motion for page transitions and micro-interactions
- Dark theme by default, glass-morphism accents

---

## 4. Data Flow

```
Smart Meters ──(NATS)──▶ Edge Nodes ──(NATS)──▶ API Gateway
     │                      │                      │
     │                      │                      ├──▶ TimescaleDB (store)
     │                      │                      ├──▶ Dashboard (WebSocket)
     │                      │                      └──▶ EDA Engine (trigger)
     │                      │
     │                      └──▶ DSL Engine (evaluate rules)
     │                              │
     │                              └──▶ Shed/Protect Commands ──▶ Meters
     │
     └──▶ Blockchain Service (surplus/deficit events)
                  │
                  └──▶ Smart Contracts (mint/trade/settle)
                          │
                          └──▶ Dashboard (trade events via WebSocket)
```

**Real-time loop (sub-second):**
1. Meter publishes reading → NATS
2. Edge node aggregates → publishes to cloud
3. API gateway persists to TimescaleDB + pushes to dashboard via WebSocket
4. EDA engine runs periodic batch (every 5 min) → publishes forecast
5. DSL engine evaluates rules against latest state → publishes commands
6. Blockchain listens for surplus/deficit → executes trades

---

## 5. DSL Design

**Grammar (PEG):**
```
Program    ← Rule+
Rule       ← "rule" STRING "{" When Action Priority "}"
When       ← "when:" Condition (AND Condition)*
Condition  ← Expr (COMP Expr)
Expr       ← Field / Literal / FuncCall
Action     ← "action:" FuncCall
Priority   ← "priority:" INT
```

**Built-in functions:** `shed(percent, duration)`, `protect()`, `notify(message)`, `curtail(percent)`

**Runtime:**
- Rules evaluated every tick (configurable, default 5s)
- State sourced from TimescaleDB latest readings + forecast
- Conflict resolution: lowest priority number wins
- Dry-run mode: log what *would* happen without executing

---

## 6. Blockchain Trading Design

**Token economics:**
- 1 EnergyToken = 1 kWh
- Initial supply: 0 (minted on surplus production)
- Price determined by marketplace (supply/demand)

**Smart contract events (consumed by dashboard):**
- `EnergyMinted(meter, amount)`
- `EnergyListed(meter, amount, price)`
- `EnergyPurchased(buyer, seller, amount, price)`
- `SettlementCompleted(buyer, seller, amount, totalPrice)`

**Security:**
- Reentrancy guards on all state-changing functions
- Access control: only edge service can mint
- Pausable trading for emergency shutdown

---

## 7. Dashboard UI Design

**Design tokens:**
```css
--color-bg-primary: #0a0a0f;
--color-bg-card: #12121a;
--color-bg-glass: rgba(18, 18, 26, 0.7);
--color-accent: #6366f1;        /* Indigo */
--color-accent-glow: rgba(99, 102, 241, 0.3);
--color-success: #22c55e;
--color-warning: #f59e0b;
--color-danger: #ef4444;
--color-text-primary: #f1f5f9;
--color-text-secondary: #94a3b8;
--radius-sm: 8px;
--radius-md: 12px;
--radius-lg: 16px;
--shadow-glow: 0 0 20px var(--color-accent-glow);
```

**Key UI patterns:**
- Glass-morphism cards with backdrop-blur
- Smooth number transitions (count-up/count-down animations)
- Sparklines embedded in table rows
- Real-time grid topology map (SVG, animated power flow lines)
- Skeleton loading states (no spinners)
- Responsive: desktop-first, but functional on tablet

---

## 8. Project Management — WBS & EVA

**WBS (Work Breakdown Structure):**

```
1.0 Smart Grid System
├── 1.1 Infrastructure
│   ├── 1.1.1 NATS setup
│   ├── 1.1.2 TimescaleDB setup
│   └── 1.1.3 Docker Compose orchestration
├── 1.2 Smart Meter Simulators
│   ├── 1.2.1 Go meter program
│   ├── 1.2.2 Docker packaging
│   └── 1.2.3 Profile configurations
├── 1.3 Edge Computing Service
│   ├── 1.3.1 Aggregation engine
│   ├── 1.3.2 Circuit breaker
│   └── 1.3.3 Command executor
├── 1.4 EDA & Forecasting
│   ├── 1.4.1 Data pipeline
│   ├── 1.4.2 EDA notebook
│   └── 1.4.3 Forecast model
├── 1.5 DSL Engine
│   ├── 1.5.1 PEG parser
│   ├── 1.5.2 Rule evaluator
│   └── 1.5.3 Hot-reload
├── 1.6 Blockchain Trading
│   ├── 1.6.1 EnergyToken contract
│   ├── 1.6.2 Marketplace contract
│   ├── 1.6.3 Settlement contract
│   └── 1.6.4 Test suite
├── 1.7 Cloud Dashboard
│   ├── 1.7.1 Layout & navigation
│   ├── 1.7.2 Real-time charts
│   ├── 1.7.3 Trading UI
│   ├── 1.7.4 DSL editor
│   └── 1.7.5 WBS/EVA panel
└── 1.8 Integration & Demo
    ├── 1.8.1 End-to-end pipeline
    ├── 1.8.2 Demo script
    └── 1.8.3 Documentation
```

**EVA Metrics (tracked in dashboard WBS/EVA panel):**
- **PV** (Planned Value): Budget allocated for work scheduled
- **EV** (Earned Value): Budget for work completed
- **AC** (Actual Cost): Actual spend (hours × rate)
- **CPI** = EV/AC (Cost Performance Index — >1 is under budget)
- **SPI** = EV/PV (Schedule Performance Index — >1 is ahead of schedule)

---

## 9. Tech Stack Summary

| Layer | Technology | Purpose |
|-------|-----------|---------|
| Simulators | Go + Docker | Smart meter fleet |
| Message Bus | NATS | Pub/sub between all services |
| Edge Computing | Go | Aggregation, local processing |
| Time-Series DB | TimescaleDB | Meter readings, forecasts |
| Relational DB | PostgreSQL | Users, contracts, metadata |
| EDA/Forecasting | Python + Pandas + Prophet | Demand prediction |
| DSL Engine | Go + PEG parser | Load-shedding rules |
| Blockchain | Hardhat + Solidity | Energy trading contracts |
| API Gateway | Go (Gin/Fiber) | REST + WebSocket bridge |
| Dashboard | Next.js 14 + Tailwind + Recharts/D3 | Cloud visualization |
| Orchestration | Docker Compose | All services in one command |

---

## 10. Non-Functional Requirements

- **Latency:** Meter → Dashboard < 2s end-to-end
- **Throughput:** 50+ simulated meters, 1 reading/sec each
- **Availability:** Circuit-breaker on edge nodes, graceful degradation on dashboard
- **Security:** Input validation on all API endpoints, reentrancy guards on contracts
- **Demo-ability:** Single `docker compose up` brings up entire system
