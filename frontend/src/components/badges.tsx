export function SeverityBadge({ severity }: { severity: string }) {
  return <span className={`sev sev-${severity.toLowerCase()}`}>{severity}</span>;
}

export function StatusBadge({ status }: { status: string }) {
  return <span className={`status status-${status.toLowerCase()}`}>{status}</span>;
}

export function ConfidenceBar({ value }: { value: number }) {
  const pct = Math.round(value * 100);
  return (
    <div className="conf" title={`confidence ${pct}%`}>
      <div className="conf-track">
        <div className="conf-fill" style={{ width: `${pct}%` }} />
      </div>
      <span className="conf-pct">{pct}%</span>
    </div>
  );
}
