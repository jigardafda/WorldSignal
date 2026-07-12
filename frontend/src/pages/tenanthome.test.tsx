import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../test/utils";

const navigate = vi.fn();
const { apiMock, authMock } = vi.hoisted(() => ({
  apiMock: { myApiKeys: vi.fn(), signals: vi.fn() },
  authMock: {
    user: { id: "t", email: "t@acme.com", name: "T", role: "ADMIN", accountId: "a1", account: { id: "a1", name: "Acme Corp", slug: "acme", status: "ACTIVE", plan: "PRO" } },
    loading: false, hasPerm: () => true, login: vi.fn(), logout: vi.fn(), refresh: vi.fn(),
  },
}));
vi.mock("../lib/api", () => ({ api: apiMock }));
vi.mock("../lib/auth", () => ({ useAuth: () => authMock }));
vi.mock("react-router-dom", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-router-dom")>();
  return { ...actual, useNavigate: () => navigate };
});

import { TenantHome } from "./TenantHome";

const signal = () => ({ id: "s1", title: "Major earthquake", summary: "", status: "CONFIRMED", severity: "HIGH", confidence: 0.82, sourceCount: 3, firstSeenAt: "2026-01-01T00:00:00Z", lastSeenAt: "2026-01-01T00:00:00Z", tags: [], sources: [] });

afterEach(() => vi.clearAllMocks());
beforeEach(() => {
  apiMock.myApiKeys.mockResolvedValue([{ id: "k1", requestCount: 40 }, { id: "k2", requestCount: 2 }]);
  apiMock.signals.mockResolvedValue([signal()]);
});

describe("TenantHome (customer console)", () => {
  it("shows plan, API usage and latest signals", async () => {
    renderWithProviders(<TenantHome />);
    // workspace subtitle
    expect(await screen.findByText(/Acme Corp · your WorldSignal workspace/)).toBeInTheDocument();
    // plan + status tiles
    expect(screen.getByText("PRO")).toBeInTheDocument();
    expect(screen.getByText("ACTIVE")).toBeInTheDocument();
    // API usage aggregates from the tenant's keys (40 + 2 = 42, and 2 keys)
    await waitFor(() => expect(screen.getByText("42")).toBeInTheDocument());
    expect(screen.getByText("2")).toBeInTheDocument();
    // latest relevant signal
    expect(await screen.findByText("Major earthquake")).toBeInTheDocument();
  });

  it("quick actions navigate to the tenant surfaces", async () => {
    renderWithProviders(<TenantHome />);
    await screen.findByText("Major earthquake");
    await userEvent.click(screen.getByTestId("qa-keys"));
    expect(navigate).toHaveBeenCalledWith("/my-api-keys");
    await userEvent.click(screen.getByTestId("qa-personalize"));
    expect(navigate).toHaveBeenCalledWith("/my-subscriptions");
    await userEvent.click(screen.getByTestId("qa-signals"));
    expect(navigate).toHaveBeenCalledWith("/signals");
  });

  it("renders even before the account is loaded", async () => {
    authMock.user = { id: "t", email: "t@acme.com", name: "T", role: "ADMIN", accountId: "a1", account: undefined } as never;
    apiMock.myApiKeys.mockResolvedValue([]);
    renderWithProviders(<TenantHome />);
    expect(await screen.findByText("Your WorldSignal workspace")).toBeInTheDocument();
    authMock.user = { id: "t", email: "t@acme.com", name: "T", role: "ADMIN", accountId: "a1", account: { id: "a1", name: "Acme Corp", slug: "acme", status: "ACTIVE", plan: "PRO" } };
  });
});
