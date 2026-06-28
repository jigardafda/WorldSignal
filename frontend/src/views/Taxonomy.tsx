import { useEffect, useState } from "react";
import { api, type TaxonomyNode } from "../api";

export function Taxonomy() {
  const [nodes, setNodes] = useState<TaxonomyNode[]>([]);
  useEffect(() => {
    api.taxonomy().then(setNodes);
  }, []);

  return (
    <div>
      <h1>WorldSignal Taxonomy</h1>
      <p className="muted">
        Closed, controlled vocabulary. The LLM (or keyword fallback) may only assign codes from this
        list — it never invents new tags.
      </p>
      <div className="tax-grid">
        {nodes.map((domain) => (
          <div className="tax-domain" key={domain.code}>
            <div className="tax-domain-head">{domain.label}</div>
            <div className="tax-children">
              {(domain.children ?? []).map((c) => (
                <span className="chip" key={c.code} title={c.label}>{c.code}</span>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
