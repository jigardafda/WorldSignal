import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { MantineProvider } from "@mantine/core";
import { render } from "@testing-library/react";
import { Layout } from "./Layout";

const { authMock } = vi.hoisted(() => ({
  authMock: { user: { id: "me", email: "me@x.io", name: "Me", role: "ADMIN" }, loading: false, logout: vi.fn(), login: vi.fn(), refresh: vi.fn(), hasPerm: (_p: string): boolean => true },
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
