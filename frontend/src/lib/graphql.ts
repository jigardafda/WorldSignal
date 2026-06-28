// Minimal typed GraphQL client with bearer-token auth and error normalization.

const TOKEN_KEY = "ws_token";

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string | null): void {
  if (token) localStorage.setItem(TOKEN_KEY, token);
  else localStorage.removeItem(TOKEN_KEY);
}

/** A normalized GraphQL/transport error. `unauthenticated` marks an expired/absent session. */
export class GqlError extends Error {
  readonly unauthenticated: boolean;
  readonly forbidden: boolean;
  constructor(message: string) {
    super(message);
    this.name = "GqlError";
    this.unauthenticated = message.includes("unauthenticated");
    this.forbidden = message.includes("forbidden");
  }
}

export async function gql<T>(query: string, variables?: Record<string, unknown>): Promise<T> {
  let res: Response;
  try {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      Accept: "application/json",
    };
    const token = getToken();
    if (token) headers.Authorization = `Bearer ${token}`;
    res = await fetch("/graphql", {
      method: "POST",
      headers,
      body: JSON.stringify({ query, variables }),
    });
  } catch {
    throw new GqlError("Network error: could not reach the server");
  }
  let json: { data?: T; errors?: { message: string }[] };
  try {
    json = await res.json();
  } catch {
    throw new GqlError("Invalid server response");
  }
  if (json.errors?.length) throw new GqlError(json.errors[0].message);
  if (json.data === undefined || json.data === null) throw new GqlError("Empty response");
  return json.data;
}
