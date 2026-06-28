import { useState } from "react";
import { Dashboard } from "./views/Dashboard";
import { Signals } from "./views/Signals";
import { Sources } from "./views/Sources";
import { Taxonomy } from "./views/Taxonomy";

type Tab = "dashboard" | "signals" | "sources" | "taxonomy";

const TABS: { key: Tab; label: string }[] = [
  { key: "dashboard", label: "Dashboard" },
  { key: "signals", label: "Signal Explorer" },
  { key: "sources", label: "Sources" },
  { key: "taxonomy", label: "Taxonomy" },
];

export default function App() {
  const [tab, setTab] = useState<Tab>("dashboard");
  return (
    <div className="app">
      <aside className="sidebar">
        <div className="brand">
          <span className="brand-dot" />
          World<strong>Signal</strong>
        </div>
        <nav>
          {TABS.map((t) => (
            <button
              key={t.key}
              className={tab === t.key ? "nav active" : "nav"}
              onClick={() => setTab(t.key)}
            >
              {t.label}
            </button>
          ))}
        </nav>
        <div className="sidebar-foot">global event intelligence fabric</div>
      </aside>
      <main className="main">
        {tab === "dashboard" && <Dashboard />}
        {tab === "signals" && <Signals />}
        {tab === "sources" && <Sources />}
        {tab === "taxonomy" && <Taxonomy />}
      </main>
    </div>
  );
}
