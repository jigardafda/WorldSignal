import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AuthProvider, useAuth } from "./auth";
import { getToken, setToken } from "./graphql";

vi.mock("./api", () => ({ api: { me: vi.fn(), login: vi.fn(), logout: vi.fn() } }));
import { api } from "./api";

const mockApi = api as unknown as {
  me: ReturnType<typeof vi.fn>;
  login: ReturnType<typeof vi.fn>;
  logout: ReturnType<typeof vi.fn>;
};

function Probe() {
  const { user, loading, login, logout, hasPerm } = useAuth();
  if (loading) return <div>loading</div>;
  return (
    <div>
      <span data-testid="user">{user ? user.email : "anon"}</span>
      <span data-testid="perm">{hasPerm("users:manage") ? "can" : "cannot"}</span>
      <button onClick={() => login("a@b.c", "pw")}>login</button>
      <button onClick={() => logout()}>logout</button>
    </div>
  );
}

function setup() {
  return render(<AuthProvider><Probe /></AuthProvider>);
}

afterEach(() => {
  localStorage.clear();
  vi.clearAllMocks();
});

describe("AuthProvider", () => {
  it("is anonymous without a token", async () => {
    setup();
    await waitFor(() => expect(screen.getByTestId("user")).toHaveTextContent("anon"));
  });

  it("loads the current user when a token exists", async () => {
    setToken("t");
    mockApi.me.mockResolvedValue({ email: "x@y.z", role: "ADMIN", permissions: ["users:manage"] });
    setup();
    await waitFor(() => expect(screen.getByTestId("user")).toHaveTextContent("x@y.z"));
    expect(screen.getByTestId("perm")).toHaveTextContent("can");
  });

  it("clears an invalid token", async () => {
    setToken("bad");
    mockApi.me.mockRejectedValue(new Error("unauthenticated"));
    setup();
    await waitFor(() => expect(screen.getByTestId("user")).toHaveTextContent("anon"));
    expect(getToken()).toBeNull();
  });

  it("logs in and out", async () => {
    mockApi.login.mockResolvedValue({ token: "newtok", user: { email: "a@b.c", role: "VIEWER", permissions: [] } });
    mockApi.me.mockResolvedValue({ email: "a@b.c", role: "VIEWER", permissions: [] });
    mockApi.logout.mockResolvedValue(true);
    setup();
    await waitFor(() => expect(screen.getByTestId("user")).toHaveTextContent("anon"));
    await userEvent.click(screen.getByText("login"));
    await waitFor(() => expect(screen.getByTestId("user")).toHaveTextContent("a@b.c"));
    expect(getToken()).toBe("newtok");
    await userEvent.click(screen.getByText("logout"));
    await waitFor(() => expect(screen.getByTestId("user")).toHaveTextContent("anon"));
    expect(getToken()).toBeNull();
  });

  it("clears locally even if logout call fails", async () => {
    mockApi.login.mockResolvedValue({ token: "t2", user: { email: "a@b.c", role: "VIEWER", permissions: [] } });
    mockApi.me.mockResolvedValue({ email: "a@b.c", role: "VIEWER", permissions: [] });
    mockApi.logout.mockRejectedValue(new Error("network"));
    setup();
    await userEvent.click(await screen.findByText("login"));
    await waitFor(() => expect(screen.getByTestId("user")).toHaveTextContent("a@b.c"));
    await userEvent.click(screen.getByText("logout"));
    await waitFor(() => expect(screen.getByTestId("user")).toHaveTextContent("anon"));
  });

  it("throws when useAuth is used outside the provider", () => {
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    function Bare() { useAuth(); return null; }
    expect(() => render(<Bare />)).toThrow(/AuthProvider/);
    spy.mockRestore();
  });
});
