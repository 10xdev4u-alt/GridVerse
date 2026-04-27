"use client";
import { useEffect, useState } from "react";

interface StatCardProps {
  label: string;
  value: number;
  unit: string;
  icon: string;
  trend?: "up" | "down";
  color?: "success" | "warning" | "danger" | "accent";
}

const colors = {
  success: "border-l-green-500",
  warning: "border-l-amber-500",
  danger: "border-l-red-500",
  accent: "border-l-indigo-500",
};

export default function StatCard({ label, value, unit, icon, trend, color = "accent" }: StatCardProps) {
  const [display, setDisplay] = useState(0);
  useEffect(() => {
    const timer = setTimeout(() => setDisplay(value), 100);
    return () => clearTimeout(timer);
  }, [value]);

  return (
    <div className={`glass-card border-l-2 ${colors[color]} animate-fade-in`}>
      <div className="flex items-center justify-between mb-2">
        <span className="text-xs font-medium text-text-muted uppercase tracking-wider">{label}</span>
        <span className="text-lg">{icon}</span>
      </div>
      <div className="flex items-baseline gap-1">
        <span className="text-2xl font-bold text-text-primary tabular-nums">
          {display.toFixed(1)}
        </span>
        <span className="text-sm text-text-muted">{unit}</span>
      </div>
      {trend && (
        <span className={`text-xs ${trend === "up" ? "text-success" : "text-danger"}`}>
          {trend === "up" ? "↑" : "↓"} 2.4%
        </span>
      )}
    </div>
  );
}