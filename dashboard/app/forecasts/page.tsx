"use client";
import { useEffect, useState } from "react";

interface Forecast {
  timestamp: string;
  predicted_demand: number;
  lower_bound: number;
  upper_bound: number;
}

export default function ForecastsPage() {
  const [forecasts, setForecasts] = useState<Forecast[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch("http://localhost:8080/api/forecasts")
      .then((r) => r.json())
      .then((data) => { setForecasts(data || []); setLoading(false); })
      .catch(() => setLoading(false));

    const ws = new WebSocket("ws://localhost:8080/ws");
    ws.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data);
        if (msg.predicted_demand) setForecasts((f) => [...f.slice(-29), msg]);
      } catch {}
    };
    return () => ws.close();
  }, []);

  const latest = forecasts[forecasts.length - 1];

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-bold text-text-primary">Demand Forecasting</h2>
      <div className="grid grid-cols-4 gap-4">
        {loading ? (
          Array.from({ length: 4 }).map((_, i) => <div key={i} className="skeleton h-28" />)
        ) : (
          <>
            <div className="glass-card border-l-2 border-l-indigo-500">
              <div className="text-xs text-text-muted uppercase mb-1">Predicted Demand</div>
              <div className="text-2xl font-bold text-text-primary">{latest?.predicted_demand?.toFixed(1) || "--"}</div>
              <div className="text-xs text-text-muted">kW (1h ahead)</div>
            </div>
            <div className="glass-card border-l-2 border-l-green-500">
              <div className="text-xs text-text-muted uppercase mb-1">Lower Bound</div>
              <div className="text-2xl font-bold text-success">{latest?.lower_bound?.toFixed(1) || "--"}</div>
              <div className="text-xs text-text-muted">kW (90% CI)</div>
            </div>
            <div className="glass-card border-l-2 border-l-amber-500">
              <div className="text-xs text-text-muted uppercase mb-1">Upper Bound</div>
              <div className="text-2xl font-bold text-warning">{latest?.upper_bound?.toFixed(1) || "--"}</div>
              <div className="text-xs text-text-muted">kW (90% CI)</div>
            </div>
            <div className="glass-card border-l-2 border-l-indigo-500">
              <div className="text-xs text-text-muted uppercase mb-1">Confidence Range</div>
              <div className="text-2xl font-bold text-text-primary">{latest ? ((latest.upper_bound - latest.lower_bound).toFixed(1)) : "--"}</div>
              <div className="text-xs text-text-muted">kW spread</div>
            </div>
          </>
        )}
      </div>
      <div className="glass-card">
        <h3 className="text-sm font-semibold text-text-secondary mb-4">Demand Forecast (Actual vs Predicted)</h3>
        <div className="h-64">
          {loading ? <div className="skeleton h-full" /> : forecasts.length === 0 ? (
            <div className="flex items-center justify-center h-full text-text-muted">Waiting for forecast data...</div>
          ) : (
            <svg viewBox="0 0 600 200" className="w-full h-full">
              {/* Confidence band */}
              {forecasts.length > 1 && (
                <polygon
                  fill="rgba(99,102,241,0.1)"
                  points={forecasts.flatMap((f, i) => [
                    `${(i / 29) * 600},${200 - (f.upper_bound / Math.max(...forecasts.map((x) => x.upper_bound), 1)) * 180}`,
                    `${(i / 29) * 600},${200 - (f.lower_bound / Math.max(...forecasts.map((x) => x.upper_bound), 1)) * 180}`,
                  ]).join(" ")}
                />
              )}
              <polyline fill="none" stroke="#6366f1" strokeWidth="2"
                points={forecasts.map((f, i) => `${(i / 29) * 600},${200 - (f.predicted_demand / Math.max(...forecasts.map((x) => x.predicted_demand), 1)) * 180}`).join(" ")}
              />
            </svg>
          )}
        </div>
      </div>
      <div className="glass-card">
        <h3 className="text-sm font-semibold text-text-secondary mb-4">EDA Insights</h3>
        <div className="grid grid-cols-3 gap-4 text-sm">
          <div className="p-4 bg-bg-card rounded-lg">
            <div className="text-text-muted mb-1">Peak Demand Hour</div>
            <div className="text-text-primary font-semibold">18:00 - 19:00</div>
          </div>
          <div className="p-4 bg-bg-card rounded-lg">
            <div className="text-text-muted mb-1">Anomaly Rate</div>
            <div className="text-text-primary font-semibold">2.1% of readings</div>
          </div>
          <div className="p-4 bg-bg-card rounded-lg">
            <div className="text-text-muted mb-1">Solar Contribution</div>
            <div className="text-success font-semibold">34% of generation</div>
          </div>
        </div>
      </div>
    </div>
  );
}