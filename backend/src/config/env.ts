import { z } from "zod";

const schema = z.object({
  DATABASE_URL: z.string().min(1, "DATABASE_URL is required"),
  PORT: z.coerce.number().default(4000),
  HOST: z.string().default("0.0.0.0"),
  OPENAI_API_KEY: z.string().optional().default(""),
  OPENAI_MODEL: z.string().default("gpt-4o-mini"),
  ROLE: z.enum(["all", "api", "worker"]).default("all"),
  WEBHOOK_SIGNING_SECRET: z.string().default("change-me-in-prod"),
});

const parsed = schema.safeParse(process.env);
if (!parsed.success) {
  console.error("❌ Invalid environment:", parsed.error.flatten().fieldErrors);
  process.exit(1);
}

export const env = parsed.data;
export const hasOpenAI = env.OPENAI_API_KEY.length > 0;
