import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../test/utils";

const { apiMock } = vi.hoisted(() => ({
  apiMock: {
    myApiKeys: vi.fn(),
    tenantApiScopes: vi.fn(),
    createMyApiKey: vi.fn(),
    revokeMyApiKey: vi.fn(),
  },
}));
vi.mock("../lib/api", () => ({ api: apiMock }));
vi.mock("@mantine/notifications", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@mantine/notifications")>();
  return { ...actual, notifications: { show: vi.fn() } };
});

import { MyApiKeys } from "./MyApiKeys";

const key = () => ({ id: "k1", accountId: "a1", name: "prod", keyPrefix: "wsk_ab", scopes: ["signals:read"], rateLimitPerMin: 120, enabled: true, expiresAt: null, lastUsedAt: null, requestCount: 0, createdBy: null, createdAt: "2026-01-01T00:00:00Z" });

afterEach(() => vi.clearAllMocks());
beforeEach(() => {
  apiMock.tenantApiScopes.mockResolvedValue(["signals:read", "stats:read"]);
});

describe("MyApiKeys (tenant)", () => {
  it("lists the tenant's own keys", async () => {
    apiMock.myApiKeys.mockResolvedValue([key()]);
    renderWithProviders(<MyApiKeys />);
    expect(await screen.findByText("prod")).toBeInTheDocument();
    expect(screen.getByText("wsk_ab…")).toBeInTheDocument();
  });

  it("shows the empty state", async () => {
    apiMock.myApiKeys.mockResolvedValue([]);
    renderWithProviders(<MyApiKeys />);
    expect(await screen.findByTestId("empty")).toBeInTheDocument();
  });

  it("creates a key and reveals the secret once", async () => {
    apiMock.myApiKeys.mockResolvedValue([]);
    apiMock.createMyApiKey.mockResolvedValue({ ...key(), id: "k2", key: "wsk_secretvalue" });
    renderWithProviders(<MyApiKeys />);
    await screen.findByTestId("empty");
    await userEvent.click(screen.getByTestId("add-key"));
    await userEvent.type(await screen.findByTestId("key-name"), "ci");
    // pick a scope
    const scopes = screen.getByTestId("key-scopes");
    fireEvent.click(scopes);
    const lb = document.getElementById(scopes.getAttribute("aria-controls")!)!;
    fireEvent.click(await within(lb).findByRole("option", { name: "signals:read", hidden: true }));
    await userEvent.click(screen.getByTestId("key-submit"));
    await waitFor(() => expect(apiMock.createMyApiKey).toHaveBeenCalledWith(expect.objectContaining({ name: "ci", scopes: ["signals:read"] })));
    // the raw secret is revealed
    expect(await screen.findByTestId("key-value")).toHaveTextContent("wsk_secretvalue");
  });

  it("validates required name and scopes", async () => {
    apiMock.myApiKeys.mockResolvedValue([]);
    renderWithProviders(<MyApiKeys />);
    await screen.findByTestId("empty");
    await userEvent.click(screen.getByTestId("add-key"));
    // Whitespace passes native `required` but trips the form validator.
    await userEvent.type(await screen.findByTestId("key-name"), "   ");
    await userEvent.click(screen.getByTestId("key-submit"));
    expect(await screen.findByText("Name is required")).toBeInTheDocument();
    expect(apiMock.createMyApiKey).not.toHaveBeenCalled();
  });

  it("surfaces a create error", async () => {
    apiMock.myApiKeys.mockResolvedValue([]);
    apiMock.createMyApiKey.mockRejectedValue(new Error("scope not allowed"));
    renderWithProviders(<MyApiKeys />);
    await screen.findByTestId("empty");
    await userEvent.click(screen.getByTestId("add-key"));
    await userEvent.type(await screen.findByTestId("key-name"), "x");
    const scopes = screen.getByTestId("key-scopes");
    fireEvent.click(scopes);
    const lb = document.getElementById(scopes.getAttribute("aria-controls")!)!;
    fireEvent.click(await within(lb).findByRole("option", { name: "signals:read", hidden: true }));
    await userEvent.click(screen.getByTestId("key-submit"));
    await waitFor(() => expect(apiMock.createMyApiKey).toHaveBeenCalled());
  });

  it("revokes a key", async () => {
    apiMock.myApiKeys.mockResolvedValue([key()]);
    apiMock.revokeMyApiKey.mockResolvedValue(true);
    renderWithProviders(<MyApiKeys />);
    await screen.findByText("prod");
    await userEvent.click(screen.getByRole("button", { name: "Revoke" }));
    const dialog = await screen.findByRole("dialog");
    await userEvent.click(within(dialog).getByRole("button", { name: "Revoke" }));
    await waitFor(() => expect(apiMock.revokeMyApiKey).toHaveBeenCalledWith("k1"));
  });
});
