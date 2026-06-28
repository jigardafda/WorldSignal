import { useEffect, useState } from "react";
import { api, type Stats, type Signal } from "../api";
import { SeverityBadge, ConfidenceBar } from "../components/badges";

export function Dashboard() {
  const [stats, setStats] = useState<Stats | null>(null);
  const [recent, setRecent] = useState<Signal[]>([]);
  const [err, setErr] = useState<string | null>(null);

  async function load() {
    try {
      const [s, sig] = await Promise.all([api.stats(), api.signals({}, 8)]);
      setStats(s);
      setRecent(sig);
      setErr(null);
    } catch (e) {
      setErr((e as Error).message);
    }
  }

  useEffect(() => {
    load();
    const t = setInterval(load, 10000);
    return () => clearInterval(t);
  }, []);

  if (err) return <div className="error">Cannot reach API: {err}</div>;

  const cards = [
    { label: "Sources", value: stats?.sources },
    { label: "Articles ingested", value: stats?.articles },
    { label: "Signals", value: stats?.signals },
    { label: "Deliveries sent", value: stats?.deliveriesSent },
    { label: "Deliveries pending", value: stats?.deliveriesPending },
  ];

  return (
    <div>
      <h1>Dashboard</h1>
      <div className="cards">
        {cards.map((c) => (
          <div className="card" key={c.label}>
            <div className="card-value">{c.value ?? "–"}</div>
            <div className="card-label">{c.label}</div>
          </div>
        ))}
      </div>

      <h2>Latest signals</h2>
      <div className="list">
        {recent.length === 0 && (
          <p className="muted">
            No signals yet. They appear once the scheduler fetches sources and the pipeline runs
            (usually within a minute or two of starting the backend).
          </p>
        )}
        {recent.map((s) => (
          <div className="row" key={s.id}>
            <SeverityBadge severity={s.severity} />
            <div className="row-main">
              <div className="row-title">{s.title}</div>
              <div className="row-meta">
                {s.tags.map((t) => (
                  <span className="chip" key={t.code}>{t.code}</span>
                ))}
                <span className="muted">· {s.sourceCount} sources · {s.status}</span>
              </div>
            </div>
            <ConfidenceBar value={s.confidence} />
          </div>
        ))}
      </div>
    </div>
  );
}
