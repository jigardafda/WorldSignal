import { useEffect, useState } from "react";
import { api, type Source } from "../api";

const P_LABEL = ["P0 critical", "P1 tier-1", "P2 regional", "P3 long-tail", "P4 archival"];

export function Sources() {
  const [sources, setSources] = useState<Source[]>([]);
  const [form, setForm] = useState({ name: "", url: "", country: "", priority: "2" });
  const [msg, setMsg] = useState<string | null>(null);

  async function load() {
    setSources(await api.sources());
  }
  useEffect(() => {
    load();
  }, []);

  async function add(e: React.FormEvent) {
    e.preventDefault();
    setMsg(null);
    try {
      await api.createSource({
        name: form.name,
        url: form.url,
        country: form.country || null,
        priority: Number(form.priority),
      });
      setForm({ name: "", url: "", country: "", priority: "2" });
      await load();
      setMsg("Source added and queued for fetch.");
    } catch (e) {
      setMsg((e as Error).message);
    }
  }

  return (
    <div>
      <h1>Sources</h1>
      <form className="source-form" onSubmit={add}>
        <input placeholder="Name" value={form.name} required onChange={(e) => setForm({ ...form, name: e.target.value })} />
        <input placeholder="RSS/Atom URL" value={form.url} required onChange={(e) => setForm({ ...form, url: e.target.value })} />
        <input placeholder="Country (e.g. US)" value={form.country} onChange={(e) => setForm({ ...form, country: e.target.value })} />
        <select value={form.priority} onChange={(e) => setForm({ ...form, priority: e.target.value })}>
          {P_LABEL.map((l, i) => (
            <option key={i} value={i}>{l}</option>
          ))}
        </select>
        <button type="submit">Add source</button>
      </form>
      {msg && <div className="notice">{msg}</div>}

      <table className="table">
        <thead>
          <tr>
            <th>Name</th>
            <th>Priority</th>
            <th>Country</th>
            <th>Credibility</th>
            <th>Last success</th>
            <th>Fails</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {sources.map((s) => (
            <tr key={s.id} className={s.enabled ? "" : "disabled"}>
              <td>
                <div className="src-name">{s.name}</div>
                <a className="src-url" href={s.url} target="_blank" rel="noreferrer">{s.url}</a>
              </td>
              <td>P{s.priority}</td>
              <td>{s.country ?? "–"}</td>
              <td>{Math.round(s.credibility * 100)}%</td>
              <td>{s.lastSuccessAt ? new Date(s.lastSuccessAt).toLocaleString() : "never"}</td>
              <td>{s.failureCount}</td>
              <td className="actions">
                <button onClick={() => api.fetchSource(s.id).then(() => setMsg(`Queued ${s.name}`))}>Fetch</button>
                <button onClick={() => api.setSourceEnabled(s.id, !s.enabled).then(load)}>
                  {s.enabled ? "Disable" : "Enable"}
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
