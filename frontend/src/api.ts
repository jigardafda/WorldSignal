// GraphQL is WorldSignal's primary protocol. The console talks to /graphql for
// everything; REST (/v1/*) exists only for external webhook ingestion and the
// polling delivery channel. Vite proxies /graphql to the backend on :4000.

export interface SignalTag {
  code: string;
  label?: string;
  confidence: number;
}
export interface SignalSource {
  publisher: string;
  url: string | null;
  publishedAt: string | null;
  relation?: string;
}
export interface Signal {
  id: string;
  title: string;
  summary: string;
  whatHappened?: string | null;
  whyItMatters?: string | null;
  status: string;
  severity: string;
  confidence: number;
  eventType?: string | null;
  country?: string | null;
  sourceCount: number;
  firstSeenAt: string;
  lastSeenAt: string;
  tags: SignalTag[];
  sources: SignalSource[];
}
export interface Source {
  id: string;
  name: string;
  type: string;
  url: string;
  country?: string | null;
  priority: number;
  credibility: number;
  enabled: boolean;
  lastSuccessAt?: string | null;
  lastFailureAt?: string | null;
  failureCount: number;
}
export interface Stats {
  sources: number;
  articles: number;
  signals: number;
  deliveriesSent: number;
  deliveriesPending: number;
}
export interface TaxonomyNode {
  code: string;
  label: string;
  children?: TaxonomyNode[];
}

/** Single GraphQL transport for the whole app. */
async function gql<T>(query: string, variables?: Record<string, unknown>): Promise<T> {
  const res = await fetch("/graphql", {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify({ query, variables }),
  });
  const json = await res.json();
  if (json.errors?.length) throw new Error(json.errors[0].message);
  return json.data as T;
}

const SIGNAL_FIELDS = `
  id title summary whatHappened whyItMatters status severity confidence
  eventType country sourceCount firstSeenAt lastSeenAt
  tags { code confidence }
  sources { publisher url publishedAt }
`;

export const api = {
  stats: () => gql<{ stats: Stats }>(`{ stats }`).then((d) => d.stats),

  signals: (filter: Record<string, unknown> = {}, limit = 50) =>
    gql<{ signals: Signal[] }>(
      `query($filter: SignalFilter, $limit: Int) {
         signals(filter: $filter, limit: $limit) { ${SIGNAL_FIELDS} }
       }`,
      { filter, limit },
    ).then((d) => d.signals),

  signal: (id: string) =>
    gql<{ signal: Signal }>(`query($id: ID!){ signal(id: $id) { ${SIGNAL_FIELDS} } }`, { id }).then(
      (d) => d.signal,
    ),

  sources: () =>
    gql<{ sources: Source[] }>(
      `{ sources { id name type url country priority credibility enabled lastSuccessAt lastFailureAt failureCount } }`,
    ).then((d) => d.sources),

  createSource: (input: Record<string, unknown>) =>
    gql<{ createSource: Source }>(
      `mutation($input: CreateSourceInput!){ createSource(input: $input) { id name } }`,
      { input },
    ).then((d) => d.createSource),

  setSourceEnabled: (id: string, enabled: boolean) =>
    gql<{ setSourceEnabled: Source }>(
      `mutation($id: ID!, $enabled: Boolean!){ setSourceEnabled(id: $id, enabled: $enabled) { id enabled } }`,
      { id, enabled },
    ).then((d) => d.setSourceEnabled),

  fetchSource: (id: string) =>
    gql<{ triggerFetch: boolean }>(`mutation($id: ID!){ triggerFetch(id: $id) }`, { id }).then(
      (d) => ({ queued: d.triggerFetch }),
    ),

  taxonomy: () => gql<{ taxonomy: TaxonomyNode[] }>(`{ taxonomy }`).then((d) => d.taxonomy),
};
