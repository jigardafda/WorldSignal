import { useEffect, useState } from "react";
import { api, type Signal } from "../api";
import { SeverityBadge, StatusBadge, ConfidenceBar } from "../components/badges";

export function Signals() {
  const [signals, setSignals] = useState<Signal[]>([]);
  const [selected, setSelected] = useState<Signal | null>(null);
  const [search, setSearch] = useState("");
  const [minConf, setMinConf] = useState("0");
  const [loading, setLoading] = useState(false);

  async function load() {
    setLoading(true);
    const filter: Record<string, unknown> = {};
    if (search) filter.search = search;
    if (Number(minConf) > 0) filter.minConfidence = Number(minConf);
    try {
      setSignals(await api.signals(filter, 100));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [minConf]);

  return (
    <div className="split">
      <div className="split-left">
        <h1>Signal Explorer</h1>
        <div className="toolbar">
          <input
            placeholder="Search signals…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && load()}
          />
          <select value={minConf} onChange={(e) => setMinConf(e.target.value)}>
            <option value="0">Any confidence</option>
            <option value="0.5">≥ 50%</option>
            <option value="0.7">≥ 70%</option>
            <option value="0.85">≥ 85%</option>
          </select>
          <button onClick={load}>{loading ? "…" : "Search"}</button>
        </div>
        <div className="list">
          {signals.map((s) => (
            <div
              key={s.id}
              className={selected?.id === s.id ? "row clickable sel" : "row clickable"}
              onClick={() => setSelected(s)}
            >
              <SeverityBadge severity={s.severity} />
              <div className="row-main">
                <div className="row-title">{s.title}</div>
                <div className="row-meta">
                  {s.tags.map((t) => (
                    <span className="chip" key={t.code}>{t.code}</span>
                  ))}
                  <span className="muted">· {s.sourceCount} src</span>
                </div>
              </div>
              <ConfidenceBar value={s.confidence} />
            </div>
          ))}
          {!loading && signals.length === 0 && <p className="muted">No matching signals.</p>}
        </div>
      </div>

      <div className="split-right">
        {!selected ? (
          <p className="muted">Select a signal to inspect its sources and enrichment.</p>
        ) : (
          <div className="detail">
            <div className="detail-head">
              <SeverityBadge severity={selected.severity} />
              <StatusBadge status={selected.status} />
            </div>
            <h2>{selected.title}</h2>
            <ConfidenceBar value={selected.confidence} />
            <p>{selected.summary}</p>
            {selected.whyItMatters && (
              <>
                <h4>Why it matters</h4>
                <p>{selected.whyItMatters}</p>
              </>
            )}
            <h4>Tags</h4>
            <div className="row-meta">
              {selected.tags.map((t) => (
                <span className="chip" key={t.code}>
                  {t.code} · {Math.round(t.confidence * 100)}%
                </span>
              ))}
            </div>
            <h4>Sources ({selected.sources.length})</h4>
            <ul className="sources">
              {selected.sources.map((src, i) => (
                <li key={i}>
                  <span className="src-rel">{src.relation}</span>
                  {src.url ? (
                    <a href={src.url} target="_blank" rel="noreferrer">{src.publisher}</a>
                  ) : (
                    <span>{src.publisher}</span>
                  )}
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>
    </div>
  );
}
