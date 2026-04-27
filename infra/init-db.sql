-- Enable TimescaleDB extension
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- Meter readings hypertable
CREATE TABLE IF NOT EXISTS meter_readings (
    time        TIMESTAMPTZ NOT NULL,
    meter_id    VARCHAR(50) NOT NULL,
    profile     VARCHAR(50) NOT NULL,
    consumption_kw  DOUBLE PRECISION,
    production_kw   DOUBLE PRECISION,
    voltage     DOUBLE PRECISION,
    status      VARCHAR(20) DEFAULT 'normal'
);

SELECT create_hypertable('meter_readings', 'time', if_not_exists => TRUE);

-- Create index on meter_id for fast lookups
CREATE INDEX IF NOT EXISTS idx_meter_id ON meter_readings (meter_id, time DESC);

-- Aggregated readings table (edge node output)
CREATE TABLE IF NOT EXISTS edge_aggregated (
    time            TIMESTAMPTZ NOT NULL,
    edge_id         VARCHAR(50) NOT NULL,
    meter_count     INTEGER,
    avg_consumption DOUBLE PRECISION,
    avg_production  DOUBLE PRECISION,
    peak_detected   BOOLEAN DEFAULT FALSE,
    anomaly_count   INTEGER DEFAULT 0
);

SELECT create_hypertable('edge_aggregated', 'time', if_not_exists => TRUE);

-- Forecast table
CREATE TABLE IF NOT EXISTS forecasts (
    time            TIMESTAMPTZ NOT NULL,
    predicted_demand DOUBLE PRECISION,
    lower_bound     DOUBLE PRECISION,
    upper_bound     DOUBLE PRECISION,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

SELECT create_hypertable('forecasts', 'time', if_not_exists => TRUE);

-- Blockchain trade events log
CREATE TABLE IF NOT EXISTS trade_events (
    time        TIMESTAMPTZ DEFAULT NOW(),
    event_type  VARCHAR(50) NOT NULL,
    buyer       VARCHAR(50),
    seller      VARCHAR(50),
    amount_kwh  DOUBLE PRECISION,
    price_per_kwh DOUBLE PRECISION,
    tx_hash     VARCHAR(66)
);

SELECT create_hypertable('trade_events', 'time', if_not_exists => TRUE);

-- WBS/EVA tracking table
CREATE TABLE IF NOT EXISTS eva_metrics (
    id          SERIAL PRIMARY KEY,
    task_name   VARCHAR(200) NOT NULL,
    pv          DOUBLE PRECISION DEFAULT 0,  -- Planned Value
    ev          DOUBLE PRECISION DEFAULT 0,  -- Earned Value
    ac          DOUBLE PRECISION DEFAULT 0,  -- Actual Cost
    status      VARCHAR(20) DEFAULT 'pending',
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);