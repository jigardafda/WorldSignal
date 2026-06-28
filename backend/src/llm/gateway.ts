import OpenAI from "openai";
import { env, hasOpenAI } from "../config/env.js";
import { logger } from "../lib/logger.js";

const log = logger("llm");

// Single place that talks to OpenAI. Business logic depends on this gateway,
// never on the OpenAI SDK directly — so swapping providers is a one-file change.

let client: OpenAI | null = null;
function getClient(): OpenAI {
  if (!client) client = new OpenAI({ apiKey: env.OPENAI_API_KEY });
  return client;
}

export interface JsonCompletionOptions {
  system: string;
  user: string;
  /** soft cap; OpenAI param */
  maxTokens?: number;
}

/**
 * Calls OpenAI expecting a strict JSON object back. Returns null when no API key
 * is configured or the call fails — callers MUST provide a deterministic fallback.
 */
export async function jsonCompletion<T>(opts: JsonCompletionOptions): Promise<T | null> {
  if (!hasOpenAI) return null;
  try {
    const res = await getClient().chat.completions.create({
      model: env.OPENAI_MODEL,
      temperature: 0.1,
      max_tokens: opts.maxTokens ?? 600,
      response_format: { type: "json_object" },
      messages: [
        { role: "system", content: opts.system },
        { role: "user", content: opts.user },
      ],
    });
    const content = res.choices[0]?.message?.content;
    if (!content) return null;
    return JSON.parse(content) as T;
  } catch (err) {
    log.warn("OpenAI call failed, falling back to heuristic", (err as Error).message);
    return null;
  }
}
