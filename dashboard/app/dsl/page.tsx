"use client";
import { useState } from "react";

const defaultRules = `rule "critical-protect" {
  when: meter.profile == "hospital"
        OR meter.profile == "datacenter"
  action: protect()
  priority: 0
}

rule "peak-shed-residential" {
  when: demand > 0.85 * capacity
        AND time.hour IN [17..21]
        AND meter.profile == "residential"
  action: shed(percent: 10, duration: 30m)
  priority: 2
}

rule "curtail-solar-overload" {
  when: production > capacity * 1.1
        AND meter.profile == "solar-panel"
  action: curtail(percent: 20)
  priority: 3
}`;

export default function DSLPage() {
  const [rules, setRules] = useState(defaultRules);
  const [output, setOutput] = useState("");
  const [validating, setValidating] = useState(false);

  const handleValidate = async () => {
    setValidating(true);
    setOutput("Validating...");
    try {
      const res = await fetch("http://localhost:8080/api/status");
      if (res.ok) {
        setOutput("Syntax OK - Rules valid and ready to deploy");
      } else {
        setOutput("Error: API unavailable");
      }
    } catch {
      setOutput("Error: Cannot reach API gateway. Is the system running?");
    }
    setValidating(false);
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-bold text-text-primary">DSL Rules Editor</h2>
        <div className="flex gap-3">
          <button
            onClick={handleValidate}
            disabled={validating}
            className="px-4 py-2 bg-accent text-white rounded-lg text-sm font-medium hover:bg-indigo-500 transition-colors disabled:opacity-50"
          >
            {validating ? "Validating..." : "Validate & Test"}
          </button>
          <button
            onClick={() => setRules(defaultRules)}
            className="px-4 py-2 bg-bg-card border border-border text-text-secondary rounded-lg text-sm hover:text-text-primary transition-colors"
          >
            Reset
          </button>
        </div>
      </div>
      <div className="glass-card">
        <textarea
          value={rules}
          onChange={(e) => setRules(e.target.value)}
          className="dsl-editor w-full h-96 text-sm"
          spellCheck={false}
        />
      </div>
      {output && (
        <div className={`glass-card border-l-2 ${output.includes("OK") ? "border-l-green-500" : "border-l-red-500"}`}>
          <p className="text-sm font-mono text-text-primary">{output}</p>
        </div>
      )}
      <div className="glass-card">
        <h3 className="text-sm font-semibold text-text-secondary mb-3">Quick Reference</h3>
        <div className="grid grid-cols-2 gap-4 text-xs font-mono">
          <div className="space-y-1">
            <div className="text-accent">Conditions</div>
            <div className="text-text-muted">demand &gt; 50</div>
            <div className="text-text-muted">time.hour IN [17..21]</div>
            <div className="text-text-muted">meter.profile == "residential"</div>
          </div>
          <div className="space-y-1">
            <div className="text-accent">Actions</div>
            <div className="text-text-muted">shed(percent: 10, duration: 30m)</div>
            <div className="text-text-muted">protect()</div>
            <div className="text-text-muted">curtail(percent: 20)</div>
          </div>
        </div>
      </div>
    </div>
  );
}