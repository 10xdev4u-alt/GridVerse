"use client";
import { useEffect, useState } from "react";

export default function StatusBar() {
  const [status, setStatus] = useState({ meters: 0, load: 0, blockchain: false });

  useEffect(() => {
    const ws = new WebSocket("ws://localhost:8080/ws");
    ws.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data);
        if (data.meter_count) setStatus((s) => ({ ...s, meters: data.meter_count }));
        if (data.avg_consumption) setStatus((s) => ({ ...s, load: data.avg_consumption }));
      } catch {}
    };
    // Also fetch status
    fetch("http://localhost:8080/api/status")
      .then((r) => r.json())
      .then((d) => setStatus((s) => ({ ...s, blockchain: d.nats })))
      .catch(() => {});
    return () => ws.close();
  }, []);

  return (
    <header className="h-14 glass border-b border-border flex items-center px-6 gap-6 sticky top-0 z-40">
      <div className="flex items-center gap-2 text-sm">
        <span className="status-dot online" />
        <span className="text-text-secondary">Meters:</span>
        <span className="font-semibold text-text-primary">{status.meters}</span>
      </div>
      <div className="flex items-center gap-2 text-sm">
        <span className="text-text-secondary">Grid Load:</span>
        <span className="font-semibold text-text-primary">{status.load.toFixed(1)} kW</span>
      </div>
      <div className="flex items-center gap-2 text-sm ml-auto">
        <span className={`status-dot ${status.blockchain ? "online" : "offline"}`} />
        <span className="text-text-secondary">
          Blockchain: {status.blockchain ? "Connected" : "Offline"}
        </span>
      </div>
    </header>
  );
}