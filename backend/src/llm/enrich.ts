import { z } from "zod";
import { jsonCompletion } from "./gateway.js";
import { leafTags, VALID_CODES, FALLBACK_CODE } from "../taxonomy/taxonomy.js";
import { firstSentences, normalizeText } from "../lib/text.js";

export type Severity = "LOW" | "MEDIUM" | "HIGH" | "CRITICAL";

export interface EnrichmentResult {
  title: string;
  summary: string;
  whatHappened: string;
  whyItMatters: string;
  severity: Severity;
  confidence: number;
  tags: { code: string; confidence: number }[];
  source: "llm" | "heuristic";
}

const llmSchema = z.object({
  title: z.string(),
  summary: z.string(),
  whatHappened: z.string().optional().default(""),
  whyItMatters: z.string().optional().default(""),
  severity: z.enum(["LOW", "MEDIUM", "HIGH", "CRITICAL"]).optional().default("MEDIUM"),
  confidence: z.number().min(0).max(1).optional().default(0.6),
  tags: z
    .array(z.object({ code: z.string(), confidence: z.number().min(0).max(1) }))
    .optional()
    .default([]),
});

function buildTaxonomyList(): string {
  return leafTags()
    .map((t) => `- ${t.code} (${t.label})`)
    .join("\n");
}

export interface EnrichInput {
  title: string;
  body: string;
  publisher?: string;
}

export async function enrichArticle(input: EnrichInput): Promise<EnrichmentResult> {
  const llm = await runLlm(input);
  if (llm) return llm;
  return heuristic(input);
}

async function runLlm(input: EnrichInput): Promise<EnrichmentResult | null> {
  const system = [
    "You are an analyst that turns a news article into a canonical event Signal.",
    "Return JSON only. Do not invent facts not present in the article.",
    "Choose tags ONLY from the provided taxonomy. Never create new tag codes.",
    "If nothing fits, use GENERAL.OTHER.",
    "",
    "Taxonomy:",
    buildTaxonomyList(),
  ].join("\n");

  const user = [
    "Produce JSON with keys: title, summary, whatHappened, whyItMatters,",
    "severity (LOW|MEDIUM|HIGH|CRITICAL), confidence (0..1),",
    "tags (array of {code, confidence}). Max 3 tags.",
    "",
    `PUBLISHER: ${input.publisher ?? "unknown"}`,
    `TITLE: ${input.title}`,
    `BODY: ${input.body.slice(0, 6000)}`,
  ].join("\n");

  const raw = await jsonCompletion<unknown>({ system, user, maxTokens: 700 });
  if (!raw) return null;

  const parsed = llmSchema.safeParse(raw);
  if (!parsed.success) return null;

  // Hard-constrain tags to the closed taxonomy.
  const tags = parsed.data.tags
    .filter((t) => VALID_CODES.has(t.code))
    .slice(0, 3);
  if (tags.length === 0) tags.push({ code: FALLBACK_CODE, confidence: 0.4 });

  return {
    title: parsed.data.title || input.title,
    summary: parsed.data.summary,
    whatHappened: parsed.data.whatHappened,
    whyItMatters: parsed.data.whyItMatters,
    severity: parsed.data.severity,
    confidence: parsed.data.confidence,
    tags,
    source: "llm",
  };
}

// Deterministic fallback so the pipeline works with no API key.
function heuristic(input: EnrichInput): EnrichmentResult {
  const haystack = normalizeText(`${input.title} ${input.body}`);
  const scored: { code: string; confidence: number }[] = [];
  for (const tag of leafTags()) {
    const kws = tag.keywords ?? [];
    let hits = 0;
    for (const kw of kws) if (haystack.includes(kw)) hits++;
    if (hits > 0) scored.push({ code: tag.code, confidence: Math.min(0.5 + hits * 0.15, 0.9) });
  }
  scored.sort((a, b) => b.confidence - a.confidence);
  const tags = scored.slice(0, 3);
  if (tags.length === 0) tags.push({ code: FALLBACK_CODE, confidence: 0.3 });

  const severity: Severity = /earthquake|flood|cyclone|war|attack|outbreak|breach|killed|dead|critical/.test(
    haystack,
  )
    ? "HIGH"
    : "MEDIUM";

  return {
    title: input.title,
    summary: firstSentences(input.body, 2) || input.title,
    whatHappened: firstSentences(input.body, 1),
    whyItMatters: "",
    severity,
    confidence: 0.45,
    tags,
    source: "heuristic",
  };
}
