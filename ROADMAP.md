# Roadmap

This roadmap describes the direction we would like WorldSignal to take. It is
**aspirational**: items are not commitments, priorities will shift with
community feedback, and nothing here is guaranteed to ship or to ship in this
order. For concrete, dated changes see [CHANGELOG.md](CHANGELOG.md).

The organizing principle remains constant: **the durable asset is the Signal,
not the article.** Everything below serves the goal of producing higher-quality,
better-classified, more useful Signals and making them easier to operate and
consume.

## Now (near-term)

- **Source health analytics.** Per-source success/failure rates, latency,
  freshness, and cooldown visibility surfaced in the admin console to make it
  easy to spot and retire failing sources.
- **Delivery channels.** Expand beyond current outbound delivery with
  additional channels. **Email delivery and hourly/daily digests have shipped**
  (SMTP connectors with Gmail/Outlook/Zoho/SendGrid presets — see
  [docs/EMAIL.md](docs/EMAIL.md)); richer webhook payloads and chat-style
  integrations are still planned.
- **Operability improvements.** Better observability around the scheduler, job
  queue, and worker pool (metrics, structured insight into retries and
  backoff).

## Next (mid-term)

- **Pluggable enrichment providers.** A provider interface so enrichment can use
  the OpenAI integration, the deterministic heuristic fallback, or alternative
  LLM/back-ends, selectable per deployment.
- **Search.** ✅ **Shipped** — ranked Postgres full-text search over Signals and
  articles, plus a queryable entity index (search + drill-down). Further
  filtering refinements (saved searches, cross-field facets) remain planned.
- **Classification and dedup tuning.** Configurable classification taxonomies
  and improved cross-source deduplication.

## Later (long-term)

- **Multi-tenant.** Tenant isolation so a single deployment can serve multiple
  independent workspaces with their own sources, roles, and data.
- **Public/consumer APIs for Signals.** Stable, documented read APIs and feeds
  for downstream consumers of Signals.
- **Extensibility.** A clearer extension surface for custom source connectors
  and post-processing.

## Out of scope (for now)

- Becoming a general-purpose CMS or article store. WorldSignal is a Signal
  pipeline, not an article archive.

## Influencing the roadmap

Ideas and feedback are welcome in
[GitHub Discussions](https://github.com/jigardafda/WorldSignal/discussions).
See [CONTRIBUTING.md](CONTRIBUTING.md) if you would like to help build any of
this.
