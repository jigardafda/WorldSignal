import { render, screen } from "@testing-library/react";
import { MantineProvider } from "@mantine/core";
import { Notifications } from "@mantine/notifications";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import App from "./App";

const { authMock, apiMock } = vi.hoisted(() => ({
  authMock: { user: null as unknown, loading: false, hasPerm: () => true, login: vi.fn(), logout: vi.fn(), refresh: vi.fn() },
  apiMock: {
    stats: vi.fn().mockResolvedValue({ sources: 0, articles: 0, signals: 0, deliveriesSent: 0, deliveriesPending: 0 }),
    signals: vi.fn().mockResolvedValue([]),
  },
}));
vi.mock("./lib/auth", () => ({ useAuth: () => authMock, AuthProvider: ({ children }: { children: React.ReactNode }) => children }));
vi.mock("./lib/api", () => ({ api: apiMock }));

function renderApp(route: string) {
  return render(
    <MantineProvider>
      <Notifications />
      <MemoryRouter initialEntries={[route]}>
        <App />
      </MemoryRouter>
    </MantineProvider>,
  );
}

afterEach(() => { authMock.user = null; authMock.loading = false; });

describe("App routing / RequireAuth", () => {
  it("shows a loader while auth resolves", () => {
    authMock.loading = true;
    renderApp("/");
    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("redirects unauthenticated users to login", async () => {
    authMock.user = null;
    renderApp("/");
    expect(await screen.findByRole("button", { name: "Sign in" })).toBeInTheDocument();
  });

  it("renders the dashboard for authenticated users", async () => {
    authMock.user = { id: "u", email: "a@b.c", role: "ADMIN" };
    renderApp("/");
    expect(await screen.findByRole("heading", { name: "Dashboard" })).toBeInTheDocument();
  });

  it("renders the login route directly", async () => {
    renderApp("/login");
    expect(await screen.findByRole("button", { name: "Sign in" })).toBeInTheDocument();
  });

  it("redirects unknown routes to dashboard", async () => {
    authMock.user = { id: "u", email: "a@b.c", role: "ADMIN" };
    renderApp("/nope");
    expect(await screen.findByRole("heading", { name: "Dashboard" })).toBeInTheDocument();
  });
});
