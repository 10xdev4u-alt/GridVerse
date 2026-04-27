"use client";
import { useEffect, useState } from "react";

interface TradeEvent {
  event_type: string;
  buyer: string;
  seller: string;
  amount_kwh: number;
  price_per_kwh: number;
  tx_hash: string;
  time: string;
}

export default function TradingPage() {
  const [trades, setTrades] = useState<TradeEvent[]>([]);
  const [priceHistory, setPriceHistory] = useState<number[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch("http://localhost:8080/api/trades")
      .then((r) => r.json())
      .then((data) => { setTrades(data || []); setLoading(false); })
      .catch(() => setLoading(false));

    const ws = new WebSocket("ws://localhost:8080/ws");
    ws.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data);
        if (msg.event_type) {
          setTrades((t) => [msg, ...t.slice(0, 49)]);
          if (msg.price_per_kwh) setPriceHistory((p) => [...p.slice(-29), msg.price_per_kwh]);
        }
      } catch {}
    };
    return () => ws.close();
  }, []);

  const avgPrice = priceHistory.length > 0 ? priceHistory.reduce((a, b) => a + b, 0) / priceHistory.length : 0.12;

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-bold text-text-primary">Energy Marketplace</h2>
      <div className="grid grid-cols-3 gap-4">
        {loading ? (
          Array.from({ length: 3 }).map((_, i) => <div key={i} className="skeleton h-28" />)
        ) : (
          <>
            <div className="glass-card border-l-2 border-l-indigo-500">
              <div className="text-xs text-text-muted uppercase mb-1">Avg Price</div>
              <div className="text-2xl font-bold text-text-primary">${avgPrice.toFixed(3)}</div>
              <div className="text-xs text-text-muted">per kWh</div>
            </div>
            <div className="glass-card border-l-2 border-l-green-500">
              <div className="text-xs text-text-muted uppercase mb-1">24h Volume</div>
              <div className="text-2xl font-bold text-text-primary">{trades.filter((t) => t.event_type === "EnergyPurchased").reduce((s, t) => s + t.amount_kwh, 0).toFixed(1)}</div>
              <div className="text-xs text-text-muted">kWh traded</div>
            </div>
            <div className="glass-card border-l-2 border-l-amber-500">
              <div className="text-xs text-text-muted uppercase mb-1">Active Listings</div>
              <div className="text-2xl font-bold text-text-primary">{trades.filter((t) => t.event_type === "EnergyListed").length}</div>
              <div className="text-xs text-text-muted">open orders</div>
            </div>
          </>
        )}
      </div>
      <div className="grid grid-cols-2 gap-6">
        <div className="glass-card">
          <h3 className="text-sm font-semibold text-text-secondary mb-4">Price Chart (kWh)</h3>
          <div className="h-48">
            {priceHistory.length === 0 ? (
              <div className="flex items-center justify-center h-full text-text-muted">Waiting for data...</div>
            ) : (
              <svg viewBox="0 0 300 150" className="w-full h-full">
                <polyline fill="none" stroke="#6366f1" strokeWidth="2"
                  points={priceHistory.map((p, i) => `${(i / 29) * 300},${150 - (p / Math.max(...priceHistory, 0.2)) * 130}`).join(" ")}
                />
              </svg>
            )}
          </div>
        </div>
        <div className="glass-card">
          <h3 className="text-sm font-semibold text-text-secondary mb-4">Order Book</h3>
          <div className="space-y-2 text-sm">
            <div className="flex justify-between text-text-muted"><span>Price</span><span>Amount</span><span>Total</span></div>
            {[0.115, 0.118, 0.12, 0.122, 0.125].map((p, i) => (
              <div key={i} className="flex justify-between text-text-primary border-t border-border pt-2">
                <span className="text-success">${p.toFixed(3)}</span>
                <span>{(Math.random() * 5 + 1).toFixed(1)} kWh</span>
                <span>${(p * (Math.random() * 5 + 1)).toFixed(2)}</span>
              </div>
            ))}
          </div>
        </div>
      </div>
      <div className="glass-card">
        <h3 className="text-sm font-semibold text-text-secondary mb-4">Trade History</h3>
        <table className="w-full text-sm">
          <thead>
            <tr className="text-text-muted text-left"><th className="pb-2">Event</th><th className="pb-2">Buyer</th><th className="pb-2">Seller</th><th className="pb-2">Amount</th><th className="pb-2">Price</th><th className="pb-2">Tx Hash</th></tr>
          </thead>
          <tbody>
            {trades.length === 0 ? (
              <tr><td colSpan={6} className="text-text-muted py-4 text-center">No trades yet</td></tr>
            ) : trades.slice(0, 20).map((t, i) => (
              <tr key={i} className="border-t border-border">
                <td className="py-2"><span className={`badge ${t.event_type === "EnergyListed" ? "badge-neutral" : t.event_type === "EnergyPurchased" ? "badge-success" : "badge-warning"}`}>{t.event_type}</span></td>
                <td className="py-2 text-text-primary">{t.buyer || "-"}</td>
                <td className="py-2 text-text-primary">{t.seller || "-"}</td>
                <td className="py-2 text-text-primary">{t.amount_kwh?.toFixed(2)} kWh</td>
                <td className="py-2 text-text-primary">${t.price_per_kwh?.toFixed(3)}</td>
                <td className="py-2 font-mono text-text-muted text-xs">{t.tx_hash?.slice(0, 12)}...</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}