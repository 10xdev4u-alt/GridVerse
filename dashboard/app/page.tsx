"use client";
import { useEffect, useState } from "react";
import StatCard from "@/components/StatCard";
import GridMap from "@/components/GridMap";

interface TradeEvent {
  event_type: string;
  buyer: string;
  seller: string;
  amount_kwh: number;
  price_per_kwh: number;
  tx_hash: string;
}

export default function Overview() {
  const [data, setData] = useState({ generation: 0, consumption: 0, health: 98.5, trades: 0 });
  const [history, setHistory] = useState<{ time: string; gen: number; con: number }[]>([]);
  const [trades, setTrades] = useState<TradeEvent[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const ws = new WebSocket("ws://localhost:8080/ws");
    ws.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data);
        if (msg.avg_production !== undefined) {
          setData((d) => ({ ...d, generation: msg.avg_production * (msg.meter_count || 1), consumption: msg.avg_consumption * (msg.meter_count || 1) }));
          setHistory((h) => [...h.slice(-29), { time: new Date().toLocaleTimeString(), gen: msg.avg_production * (msg.meter_count || 1), con: msg.avg_consumption * (msg.meter_count || 1) }]);
        }
        if (msg.event_type) {
          setTrades((t) => [msg, ...t.slice(0, 9)]);
          setData((d) => ({ ...d, trades: d.trades + 1 }));
        }
      } catch {}
    };
    setTimeout(() => setLoading(false), 1500);
    return () => ws.close();
  }, []);

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-bold text-text-primary">Grid Overview</h2>
      <div className="grid grid-cols-4 gap-4">
        {loading ? (
          Array.from({ length: 4 }).map((_, i) => <div key={i} className="skeleton h-28" />)
        ) : (
          <>
            <StatCard label="Generation" value={data.generation} unit="kW" icon="⚡" color="success" trend="up" />
            <StatCard label="Consumption" value={data.consumption} unit="kW" icon="🏠" color="warning" trend="up" />
            <StatCard label="Grid Health" value={data.health} unit="%" icon="🛡" color="success" />
            <StatCard label="Active Trades" value={data.trades} unit="" icon="◎" color="accent" />
          </>
        )}
      </div>
      <div className="grid grid-cols-2 gap-6">
        <div className="glass-card">
          <h3 className="text-sm font-semibold text-text-secondary mb-4">Grid Topology</h3>
          <div className="h-64">{loading ? <div className="skeleton h-full" /> : <GridMap />}</div>
        </div>
        <div className="glass-card">
          <h3 className="text-sm font-semibold text-text-secondary mb-4">Generation vs Consumption</h3>
          <div className="h-64">
            {loading ? <div className="skeleton h-full" /> : (
              <svg viewBox="0 0 300 200" className="w-full h-full">
                <polyline fill="none" stroke="#22c55e" strokeWidth="2" points={history.map((p, i) => `${(i / 29) * 300},${200 - (p.gen / Math.max(...history.map((h) => h.gen), 1)) * 180}`).join(" ")} />
                <polyline fill="none" stroke="#f59e0b" strokeWidth="2" points={history.map((p, i) => `${(i / 29) * 300},${200 - (p.con / Math.max(...history.map((h) => h.con), 1)) * 180}`).join(" ")} />
              </svg>
            )}
          </div>
        </div>
      </div>
      <div className="glass-card">
        <h3 className="text-sm font-semibold text-text-secondary mb-4">Recent Trades</h3>
        {loading ? <div className="skeleton h-32" /> : (
          <table className="w-full text-sm">
            <thead>
              <tr className="text-text-muted text-left">
                <th className="pb-2">Event</th><th className="pb-2">Buyer</th><th className="pb-2">Seller</th><th className="pb-2">Amount</th><th className="pb-2">Price</th>
              </tr>
            </thead>
            <tbody>
              {trades.length === 0 ? (
                <tr><td colSpan={5} className="text-text-muted py-4 text-center">Waiting for trades...</td></tr>
              ) : trades.map((t, i) => (
                <tr key={i} className="border-t border-border">
                  <td className="py-2"><span className={`badge ${t.event_type === "EnergyListed" ? "badge-neutral" : t.event_type === "EnergyPurchased" ? "badge-success" : "badge-warning"}`}>{t.event_type}</span></td>
                  <td className="py-2 text-text-primary">{t.buyer || "-"}</td>
                  <td className="py-2 text-text-primary">{t.seller || "-"}</td>
                  <td className="py-2 text-text-primary">{t.amount_kwh?.toFixed(2)} kWh</td>
                  <td className="py-2 text-text-primary">${t.price_per_kwh?.toFixed(2)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}