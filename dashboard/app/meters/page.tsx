"use client";
import { useEffect, useState } from "react";

interface Meter {
  meter_id: string;
  profile: string;
  consumption_kw: number;
  production_kw: number;
  voltage: number;
  status: string;
  last_seen: string;
}

export default function MetersPage() {
  const [meters, setMeters] = useState<Meter[]>([]);
  const [filter, setFilter] = useState("");
  const [search, setSearch] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchMeters = () => {
      fetch("http://localhost:8080/api/meters")
        .then((r) => r.json())
        .then((data) => { setMeters(data || []); setLoading(false); })
        .catch(() => setLoading(false));
    };
    fetchMeters();
    const interval = setInterval(fetchMeters, 5000);
    return () => clearInterval(interval);
  }, []);

  const filtered = meters.filter((m) => {
    if (filter && m.profile !== filter) return false;
    if (search && !m.meter_id.includes(search)) return false;
    return true;
  });

  const profiles = [...new Set(meters.map((m) => m.profile))];

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-bold text-text-primary">Smart Meters</h2>
        <div className="flex gap-3">
          <input
            type="text"
            placeholder="Search meter ID..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="bg-bg-card border border-border rounded-lg px-3 py-2 text-sm text-text-primary placeholder-text-muted focus:outline-none focus:border-accent"
          />
          <select
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            className="bg-bg-card border border-border rounded-lg px-3 py-2 text-sm text-text-primary focus:outline-none focus:border-accent"
          >
            <option value="">All Profiles</option>
            {profiles.map((p) => <option key={p} value={p}>{p}</option>)}
          </select>
        </div>
      </div>
      <div className="glass-card overflow-x-auto">
        {loading ? (
          <div className="space-y-3">{Array.from({ length: 5 }).map((_, i) => <div key={i} className="skeleton h-12" />)}</div>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="text-text-muted text-left border-b border-border">
                <th className="pb-3">Meter ID</th><th className="pb-3">Profile</th><th className="pb-3">Consumption</th><th className="pb-3">Production</th><th className="pb-3">Voltage</th><th className="pb-3">Status</th><th className="pb-3">Last Seen</th>
              </tr>
            </thead>
            <tbody>
              {filtered.length === 0 ? (
                <tr><td colSpan={7} className="text-text-muted py-8 text-center">No meters found</td></tr>
              ) : filtered.map((m) => (
                <tr key={m.meter_id} className="border-t border-border hover:bg-white/[0.02]">
                  <td className="py-3 font-mono text-text-primary">{m.meter_id}</td>
                  <td className="py-3"><span className="badge badge-neutral">{m.profile}</span></td>
                  <td className="py-3 text-text-primary">{m.consumption_kw?.toFixed(2)} kW</td>
                  <td className="py-3 text-text-primary">{m.production_kw?.toFixed(2)} kW</td>
                  <td className="py-3 text-text-primary">{m.voltage?.toFixed(0)} V</td>
                  <td className="py-3">
                    <span className={`badge ${m.status === "normal" ? "badge-success" : m.status === "protected" ? "badge-warning" : "badge-danger"}`}>
                      {m.status}
                    </span>
                  </td>
                  <td className="py-3 text-text-muted text-xs">{new Date(m.last_seen).toLocaleTimeString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}