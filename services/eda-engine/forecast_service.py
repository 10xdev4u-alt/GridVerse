import asyncio
import json
import os
import pandas as pd
import numpy as np
from datetime import datetime, timedelta
import psycopg2
from nats.aio.client import Client as NATS

NATS_URL = os.getenv("NATS_URL", "nats://nats:4222")
DB_URL = os.getenv("DB_URL", "postgres://smartgrid:smartgrid@timescaledb:5432/smartgrid?sslmode=disable")
INTERVAL = int(os.getenv("FORECAST_INTERVAL", "300"))  # 5 min

def get_db_conn():
    return psycopg2.connect(DB_URL)

async def run_forecast():
    conn = get_db_conn()
    cur = conn.cursor()
    cur.execute("""
        SELECT time, meter_id, consumption_kw, production_kw
        FROM meter_readings
        WHERE time > NOW() - INTERVAL '24 hours'
        ORDER BY time
    """)
    rows = cur.fetchall()
    cur.close()
    conn.close()

    if len(rows) < 10:
        return None

    df = pd.DataFrame(rows, columns=["time", "meter_id", "consumption_kw", "production_kw"])
    df["time"] = pd.to_datetime(df["time"])
    df = df.set_index("time")

    # Aggregate total demand per 5-min bucket
    demand = df["consumption_kw"].resample("5min").sum().reset_index()
    demand.columns = ["ds", "y"]
    demand = demand.dropna()

    if len(demand) < 10:
        return None

    try:
        from prophet import Prophet
        m = Prophet(interval_width=0.9)
        m.fit(demand.tail(200))
        future = m.make_future_dataframe(periods=12, freq="5min")
        forecast = m.predict(future)
        last = forecast.iloc[-1]
        return {
            "timestamp": last["ds"].strftime("%Y-%m-%dT%H:%M:%SZ") if hasattr(last["ds"], 'strftime') else str(last["ds"]),
            "predicted_demand": round(last["yhat"], 2),
            "lower_bound": round(last["yhat_lower"], 2),
            "upper_bound": round(last["yhat_upper"], 2),
        }
    except Exception as e:
        print(f"Forecast error: {e}")
        # Fallback: simple moving average
        avg = demand["y"].tail(12).mean()
        return {
            "timestamp": (datetime.utcnow() + timedelta(hours=1)).strftime("%Y-%m-%dT%H:%M:%SZ"),
            "predicted_demand": round(avg, 2),
            "lower_bound": round(avg * 0.85, 2),
            "upper_bound": round(avg * 1.15, 2),
        }

async def main():
    nc = NATS()
    await nc.connect(NATS_URL)
    print(f"EDA Engine connected to NATS at {NATS_URL}")

    async def forecast_loop():
        while True:
            result = await run_forecast()
            if result:
                data = json.dumps(result).encode()
                await nc.publish("forecast.demand", data)
                print(f"Published forecast: {result}")
            await asyncio.sleep(INTERVAL)

    asyncio.create_task(forecast_loop())
    try:
        await asyncio.Event().wait()
    except KeyboardInterrupt:
        await nc.close()

if __name__ == "__main__":
    asyncio.run(main())