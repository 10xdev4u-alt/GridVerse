"use client";

const nodes = [
  { id: "edge-1", x: 150, y: 100, label: "Edge Node 1" },
  { id: "edge-2", x: 400, y: 80, label: "Edge Node 2" },
  { id: "edge-3", x: 280, y: 220, label: "Edge Node 3" },
];

const meters = [
  { id: "m1", x: 80, y: 50, edge: "edge-1" },
  { id: "m2", x: 120, y: 140, edge: "edge-1" },
  { id: "m3", x: 200, y: 60, edge: "edge-1" },
  { id: "m4", x: 340, y: 40, edge: "edge-2" },
  { id: "m5", x: 420, y: 130, edge: "edge-2" },
  { id: "m6", x: 460, y: 60, edge: "edge-2" },
  { id: "m7", x: 230, y: 180, edge: "edge-3" },
  { id: "m8", x: 310, y: 250, edge: "edge-3" },
  { id: "m9", x: 350, y: 200, edge: "edge-3" },
];

export default function GridMap() {
  return (
    <svg viewBox="0 0 550 300" className="w-full h-full">
      {/* Grid lines */}
      {nodes.map((n) =>
        meters
          .filter((m) => m.edge === n.id)
          .map((m) => (
            <line
              key={`${n.id}-${m.id}`}
              x1={n.x} y1={n.y} x2={m.x} y2={m.y}
              className="grid-line"
            />
          ))
      )}
      {/* Power flow animations */}
      {nodes.map((n) =>
        meters
          .filter((m) => m.edge === n.id)
          .map((m) => (
            <line
              key={`flow-${n.id}-${m.id}`}
              x1={n.x} y1={n.y} x2={m.x} y2={m.y}
              className="power-flow"
            />
          ))
      )}
      {/* Edge nodes */}
      {nodes.map((n) => (
        <g key={n.id}>
          <circle cx={n.x} cy={n.y} r="14" fill="#12121a" stroke="#6366f1" strokeWidth="2" />
          <text x={n.x} y={n.y + 30} textAnchor="middle" fill="#94a3b8" fontSize="10">
            {n.label}
          </text>
        </g>
      ))}
      {/* Meter nodes */}
      {meters.map((m) => (
        <g key={m.id}>
          <circle cx={m.x} cy={m.y} r="6" fill="#6366f1" className="animate-pulse-glow" />
          <text x={m.x} y={m.y - 10} textAnchor="middle" fill="#f1f5f9" fontSize="8">
            {m.id}
          </text>
        </g>
      ))}
      {/* Cloud */}
      <rect x="200" y="10" width="100" height="30" rx="6" fill="rgba(99,102,241,0.1)" stroke="rgba(99,102,241,0.3)" />
      <text x="250" y="30" textAnchor="middle" fill="#6366f1" fontSize="10">Cloud API</text>
    </svg>
  );
}