"use client";
import Link from "next/link";
import { usePathname } from "next/navigation";

const navItems = [
  { href: "/", label: "Overview", icon: "◉" },
  { href: "/meters", label: "Meters", icon: "⚡" },
  { href: "/trading", label: "Trading", icon: "◎" },
  { href: "/forecasts", label: "Forecasts", icon: "↗" },
  { href: "/dsl", label: "DSL Rules", icon: "⚙" },
  { href: "/wbs", label: "WBS/EVA", icon: "▤" },
];

export default function Sidebar() {
  const pathname = usePathname();
  return (
    <aside className="sidebar fixed left-0 top-0 h-full w-64 flex flex-col p-4 z-50">
      <div className="mb-8 px-2 pt-4">
        <h1 className="text-lg font-bold tracking-tight text-text-primary">
          Smart Grid
        </h1>
        <p className="text-xs text-text-muted mt-1">IoT Energy Platform</p>
      </div>
      <nav className="flex flex-col gap-1">
        {navItems.map((item) => (
          <Link
            key={item.href}
            href={item.href}
            className={`nav-item flex items-center gap-3 text-sm font-medium ${
              pathname === item.href ? "active" : ""
            }`}
          >
            <span className="text-base">{item.icon}</span>
            {item.label}
          </Link>
        ))}
      </nav>
      <div className="mt-auto pt-4 border-t border-border">
        <div className="flex items-center gap-2 text-xs text-text-muted px-2">
          <span className="status-dot online" />
          System Online
        </div>
      </div>
    </aside>
  );
}