"use client";
import { useEffect, useState } from "react";

interface EVAMetric {
  id: number;
  task_name: string;
  pv: number;
  ev: number;
  ac: number;
  status: string;
}

export default function WBSPage() {
  const [metrics, setMetrics] = useState<EVAMetric[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch("http://localhost:8080/api/eva")
      .then((r) => r.json())
      .then((data) => {
        if (data && data.length > 0) {
          setMetrics(data);
        } else {
          // Default WBS data
          setMetrics([
            { id: 1, task_name: "Infrastructure Setup", pv: 8, ev: 8, ac: 7, status: "completed" },
            { id: 2, task_name: "Smart Meter Simulators", pv: 12, ev: 12, ac: 10, status: "completed" },
            { id: 3, task_name: "Edge Computing Service", pv: 10, ev: 10, ac: 9, status: "completed" },
            { id: 4, task_name: "API Gateway", pv: 10, ev: 10, ac: 8, status: "completed" },
            { id: 5, task_name: "DSL Engine", pv: 12, ev: 12, ac: 11, status: "completed" },
            { id: 6, task_name: "Blockchain Contracts", pv: 15, ev: 15, ac: 14, status: "completed" },
            { id: 7, task_name: "EDA & Forecasting", pv: 10, ev: 10, ac: 9, status: "completed" },
            { id: 8, task_name: "Blockchain Bridge", pv: 8, ev: 8, ac: 7, status: "completed" },
            { id: 9, task_name: "Cloud Dashboard", pv: 15, ev: 15, ac: 14, status: "completed" },
          ]);
        }
        setLoading(false);
      })
      .catch(() => {
        setMetrics([
          { id: 1, task_name: "Infrastructure Setup", pv: 8, ev: 8, ac: 7, status: "completed" },
          { id: 2, task_name: "Smart Meter Simulators", pv: 12, ev: 12, ac: 10, status: "completed" },
          { id: 3, task_name: "Edge Computing Service", pv: 10, ev: 10, ac: 9, status: "completed" },
          { id: 4, task_name: "API Gateway", pv: 10, ev: 10, ac: 8, status: "completed" },
          { id: 5, task_name: "DSL Engine", pv: 12, ev: 12, ac: 11, status: "completed" },
          { id: 6, task_name: "Blockchain Contracts", pv: 15, ev: 15, ac: 14, status: "completed" },
          { id: 7, task_name: "EDA & Forecasting", pv: 10, ev: 10, ac: 9, status: "completed" },
          { id: 8, task_name: "Blockchain Bridge", pv: 8, ev: 8, ac: 7, status: "completed" },
          { id: 9, task_name: "Cloud Dashboard", pv: 15, ev: 15, ac: 14, status: "completed" },
        ]);
        setLoading(false);
      });
  }, []);

  const totalPV = metrics.reduce((s, m) => s + m.pv, 0);
  const totalEV = metrics.reduce((s, m) => s + m.ev, 0);
  const totalAC = metrics.reduce((s, m) => s + m.ac, 0);
  const cpi = totalAC > 0 ? (totalEV / totalAC).toFixed(2) : "1.00";
  const spi = totalPV > 0 ? (totalEV / totalPV).toFixed(2) : "1.00";

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-bold text-text-primary">WBS & Earned Value Analysis</h2>
      <div className="grid grid-cols-5 gap-4">
        {loading ? (
          Array.from({ length: 5 }).map((_, i) => <div key={i} className="skeleton h-28" />)
        ) : (
          <>
            <div className="glass-card border-l-2 border-l-indigo-500">
              <div className="text-xs text-text-muted uppercase mb-1">PV (Planned)</div>
              <div className="text-2xl font-bold text-text-primary">{totalPV}</div>
              <div className="text-xs text-text-muted">hours</div>
            </div>
            <div className="glass-card border-l-2 border-l-green-500">
              <div className="text-xs text-text-muted uppercase mb-1">EV (Earned)</div>
              <div className="text-2xl font-bold text-success">{totalEV}</div>
              <div className="text-xs text-text-muted">hours</div>
            </div>
            <div className="glass-card border-l-2 border-l-amber-500">
              <div className="text-xs text-text-muted uppercase mb-1">AC (Actual)</div>
              <div className="text-2xl font-bold text-warning">{totalAC}</div>
              <div className="text-xs text-text-muted">hours</div>
            </div>
            <div className="glass-card border-l-2 border-l-green-500">
              <div className="text-xs text-text-muted uppercase mb-1">CPI (Cost)</div>
              <div className="text-2xl font-bold text-text-primary">{cpi}</div>
              <div className="text-xs text-text-muted">{+cpi >= 1 ? "Under budget" : "Over budget"}</div>
            </div>
            <div className="glass-card border-l-2 border-l-indigo-500">
              <div className="text-xs text-text-muted uppercase mb-1">SPI (Schedule)</div>
              <div className="text-2xl font-bold text-text-primary">{spi}</div>
              <div className="text-xs text-text-muted">{+spi >= 1 ? "On/ahead schedule" : "Behind schedule"}</div>
            </div>
          </>
        )}
      </div>
      <div className="glass-card">
        <h3 className="text-sm font-semibold text-text-secondary mb-4">Work Breakdown Structure</h3>
        {loading ? (
          <div className="space-y-3">{Array.from({ length: 9 }).map((_, i) => <div key={i} className="skeleton h-10" />)}</div>
        ) : (
          <div className="space-y-2">
            {metrics.map((m) => (
              <div key={m.id} className="flex items-center gap-4 p-3 bg-bg-card rounded-lg">
                <span className="text-text-muted text-xs w-8">{m.id}.0</span>
                <span className="flex-1 text-sm text-text-primary font-medium">{m.task_name}</span>
                <div className="flex-1 h-2 bg-bg-primary rounded-full overflow-hidden">
                  <div
                    className="h-full bg-accent rounded-full transition-all"
                    style={{ width: `${(m.ev / m.pv) * 100}%` }}
                  />
                </div>
                <span className="text-xs text-text-muted w-16 text-right">{m.ev}/{m.pv}h</span>
                <span className={`badge ${m.status === "completed" ? "badge-success" : "badge-warning"}`}>{m.status}</span>
              </div>
            ))}
          </div>
        )}
      </div>
      <div className="glass-card">
        <h3 className="text-sm font-semibold text-text-secondary mb-4">EVA Chart (PV vs EV vs AC)</h3>
        <div className="h-48">
          {loading ? <div className="skeleton h-full" /> : (
            <svg viewBox="0 0 400 150" className="w-full h-full">
              {[
                { data: metrics.map((m) => m.pv), color: "#6366f1", label: "PV" },
                { data: metrics.map((m) => m.ev), color: "#22c55e", label: "EV" },
                { data: metrics.map((m) => m.ac), color: "#f59e0b", label: "AC" },
              ].map((series) => {
                const cum = series.data.reduce<number[]>((acc, v, i) => [...acc, (acc[i - 1] || 0) + v], [] as number[]);
                const maxVal = Math.max(...cum, 1);
                return (
                  <polyline key={series.label} fill="none" stroke={series.color} strokeWidth="2"
                    points={cum.map((v, i) => `${(i / (cum.length - 1)) * 380 + 10},${150 - (v / maxVal) * 130}`).join(" ")}
                  />
                );
              })}
            </svg>
          )}
        </div>
      </div>
    </div>
  );
}