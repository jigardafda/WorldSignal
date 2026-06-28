// Side-effect module imported BEFORE config/env in the OpenAI gateway test so
// that hasOpenAI evaluates true. Kept separate because static imports hoist.
process.env.OPENAI_API_KEY = "sk-test-key-not-real";
process.env.OPENAI_MODEL = "gpt-4o-mini";
