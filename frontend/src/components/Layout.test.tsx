import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { MantineProvider } from "@mantine/core";
import { render } from "@testing-library/react";
import { Layout } from "./Layout";

const { authMock } = vi.hoisted(() => ({
  authMock: { user: { id: "me", email: "me@x.io", name: "Me", role: "ADMIN" } as { id: string; email: string; name: string; role: string; accountId?: string; account?: { id: string; name: string; slug: string; status: string; plan: string } }, loading: false, logout: vi.fn(), login: vi.fn(), refresh: vi.fn(), hasPerm: (_p: string): boolean => true },
}));
vi.mock("../lib/auth", () => ({ useAuth: () => authMock }));
// Live Mode mounts Leaflet (needs a real DOM map); stub it here.
vi.mock("../pages/LiveDashboard", () => ({ LiveDashboard: () => <div data-testid="live-dashboard">live map</div> }));
import { LiveDashboard } from "../pages/LiveDashboard";

function renderLayout() {
  return render(
    <MantineProvider>
      <MemoryRouter initialEntries={["/"]}>
        <Routes>
          <Route element={<Layout />}>
            <Route index element={<div>home-content</div>} />
            <Route path="live" element={<LiveDashboard />} />
            <Route path="account" element={<div>account-page</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </MantineProvider>,
  );
}

afterEach(() => { vi.clearAllMocks(); authMock.hasPerm = () => true; });

describe("Layout", () => {
  it("renders nav, brand and outlet content", () => {
    renderLayout();
    expect(screen.getByText("home-content")).toBeInTheDocument();
    expect(screen.getAllByText("Dashboard").length).toBeGreaterThan(0); // nav item + mode toggle
    expect(screen.getByText("Users")).toBeInTheDocument();
  });

  it("shows the operator console for platform staff", () => {
    authMock.user = { id: "me", email: "me@x.io", name: "Me", role: "ADMIN" };
    renderLayout();
    expect(screen.getByTestId("console-mode")).toHaveTextContent("Operator");
    expect(screen.getByText("Sources")).toBeInTheDocument();
    expect(screen.getByText("Users")).toBeInTheDocument();
  });

  it("shows the customer console with a tenant-only menu for account users", () => {
    // An account-scoped user gets the customer console: no operator surfaces.
    authMock.user = { id: "t", email: "t@acme.com", name: "T", role: "ADMIN", accountId: "a1", account: { id: "a1", name: "Acme Corp", slug: "acme", status: "ACTIVE", plan: "PRO" } };
    renderLayout();
    expect(screen.getByTestId("console-mode")).toHaveTextContent("Customer");
    expect(screen.getByTestId("workspace-name")).toHaveTextContent("Acme Corp");
    expect(screen.getByText("Signals")).toBeInTheDocument();
    expect(screen.getByText("API Keys")).toBeInTheDocument();
    expect(screen.getByText("My Account")).toBeInTheDocument();
    // Operator-only items are absent.
    expect(screen.queryByText("Sources")).toBeNull();
    expect(screen.queryByText("Users")).toBeNull();
    expect(screen.queryByText("Accounts")).toBeNull();
    authMock.user = { id: "me", email: "me@x.io", name: "Me", role: "ADMIN" }; // restore
  });

  it("switches the color scheme from the account menu", async () => {
    const user = userEvent.setup();
    renderLayout();
    await user.click(screen.getByTestId("user-menu"));
    const toggle = await screen.findByTestId("theme-toggle");
    await user.click(within(toggle).getByText("Dark"));
    await waitFor(() =>
      expect(document.documentElement.getAttribute("data-mantine-color-scheme")).toBe("dark"),
    );
    await user.click(within(toggle).getByText("Light"));
    await waitFor(() =>
      expect(document.documentElement.getAttribute("data-mantine-color-scheme")).toBe("light"),
    );
  });

  it("toggles full-screen Live Mode from the top bar", async () => {
    renderLayout();
    expect(screen.getByText("home-content")).toBeInTheDocument();
    expect(screen.queryByTestId("live-dashboard")).toBeNull();

    await userEvent.click(screen.getByText("Live")); // only the toggle has "Live"
    expect(await screen.findByTestId("live-dashboard")).toBeInTheDocument();
    expect(screen.queryByText("home-content")).toBeNull(); // outlet replaced by full-screen map

    await userEvent.click(within(screen.getByTestId("dashboard-mode")).getByText("Dashboard"));
    expect(await screen.findByText("home-content")).toBeInTheDocument();
  });

  it("hides permission-gated nav items", () => {
    authMock.hasPerm = (p) => p !== "users:manage" && p !== "teams:manage";
    renderLayout();
    expect(screen.queryByText("Users")).toBeNull();
    expect(screen.getByText("Signals")).toBeInTheDocument();
  });

  it("opens the user menu and logs out", async () => {
    renderLayout();
    await userEvent.click(screen.getByTestId("user-menu"));
    await userEvent.click(await screen.findByText("Log out"));
    expect(authMock.logout).toHaveBeenCalled();
  });

  it("navigates to account from the menu", async () => {
    renderLayout();
    await userEvent.click(screen.getByTestId("user-menu"));
    await userEvent.click(await screen.findByText("Account"));
    await waitFor(() => expect(screen.getByText("account-page")).toBeInTheDocument());
  });

  it("navigates via a nav link", async () => {
    renderLayout();
    await userEvent.click(screen.getByText("Signals"));
    // route changes to /signals (no matching child here → outlet empty, but no crash)
  });
});
