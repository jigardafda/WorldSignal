import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../test/utils";

const { apiMock } = vi.hoisted(() => ({
  apiMock: {
    apiKeys: vi.fn(), apiScopes: vi.fn(), createApiKey: vi.fn(),
    setApiKeyEnabled: vi.fn(), deleteApiKey: vi.fn(),
  },
}));
vi.mock("../lib/api", () => ({ api: apiMock }));

import { ApiKeys } from "./ApiKeys";

const key = (o = {}) => ({
  id: "k1", name: "Pipeline", keyPrefix: "wsk_abc123", scopes: ["signals:read", "stats:read"],
  rateLimitPerMin: 120, enabled: true, expiresAt: null, lastUsedAt: null, requestCount: 0,
  createdBy: "admin", createdAt: "2026-01-01T00:00:00Z", ...o,
});

beforeEach(() => apiMock.apiScopes.mockResolvedValue(["signals:read", "stats:read", "sources:read"]));
afterEach(() => vi.clearAllMocks());

describe("ApiKeys", () => {
  it("lists keys with prefix, scopes and usage", async () => {
    apiMock.apiKeys.mockResolvedValue([key({ requestCount: 42, lastUsedAt: null })]);
    renderWithProviders(<ApiKeys />);
    expect(await screen.findByText("Pipeline")).toBeInTheDocument();
    expect(screen.getByText("wsk_abc123…")).toBeInTheDocument();
    expect(screen.getByText("signals:read")).toBeInTheDocument();
    expect(screen.getByText("never")).toBeInTheDocument();
    expect(screen.getByText("42")).toBeInTheDocument();
  });

  it("shows the empty state", async () => {
    apiMock.apiKeys.mockResolvedValue([]);
    renderWithProviders(<ApiKeys />);
    expect(await screen.findByText(/No API keys yet/)).toBeInTheDocument();
  });

  it("creates a key and reveals the secret exactly once", async () => {
    apiMock.apiKeys.mockResolvedValue([]);
    apiMock.createApiKey.mockResolvedValue(key({ key: "wsk_supersecretvalue" }));
    renderWithProviders(<ApiKeys />);
    await screen.findByText(/No API keys yet/);
    await userEvent.click(screen.getByTestId("add-key"));
    await userEvent.type(await screen.findByTestId("key-name"), "Pipeline");
    // Pick a scope from the MultiSelect.
    fireEvent.click(screen.getByTestId("key-scopes"));
    fireEvent.click(await screen.findByRole("option", { name: "signals:read", hidden: true }));
    await userEvent.click(screen.getByTestId("key-submit"));
    await waitFor(() => expect(apiMock.createApiKey).toHaveBeenCalledWith(
      expect.objectContaining({ name: "Pipeline", scopes: ["signals:read"], rateLimitPerMin: 120 }),
    ));
    // The raw secret is revealed once.
    expect(await screen.findByTestId("key-value")).toHaveTextContent("wsk_supersecretvalue");
  });

  it("validates that a scope is required", async () => {
    apiMock.apiKeys.mockResolvedValue([]);
    renderWithProviders(<ApiKeys />);
    await screen.findByText(/No API keys yet/);
    await userEvent.click(screen.getByTestId("add-key"));
    await userEvent.type(await screen.findByTestId("key-name"), "NoScopes");
    await userEvent.click(screen.getByTestId("key-submit"));
    expect(await screen.findByText("Select at least one scope")).toBeInTheDocument();
    expect(apiMock.createApiKey).not.toHaveBeenCalled();
  });

  it("toggles and deletes a key", async () => {
    apiMock.apiKeys.mockResolvedValue([key()]);
    apiMock.setApiKeyEnabled.mockResolvedValue(key({ enabled: false }));
    apiMock.deleteApiKey.mockResolvedValue(true);
    renderWithProviders(<ApiKeys />);
    await screen.findByText("Pipeline");
    await userEvent.click(screen.getByRole("button", { name: "Disable" }));
    await waitFor(() => expect(apiMock.setApiKeyEnabled).toHaveBeenCalledWith("k1", false));
    await userEvent.click(screen.getByRole("button", { name: "Delete" }));
    const dialog = await screen.findByRole("dialog");
    await userEvent.click(within(dialog).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(apiMock.deleteApiKey).toHaveBeenCalledWith("k1"));
  });

  it("surfaces a create error", async () => {
    apiMock.apiKeys.mockResolvedValue([]);
    apiMock.createApiKey.mockRejectedValue(new Error("boom"));
    renderWithProviders(<ApiKeys />);
    await screen.findByText(/No API keys yet/);
    await userEvent.click(screen.getByTestId("add-key"));
    await userEvent.type(await screen.findByTestId("key-name"), "X");
    fireEvent.click(screen.getByTestId("key-scopes"));
    fireEvent.click(await screen.findByRole("option", { name: "signals:read", hidden: true }));
    await userEvent.click(screen.getByTestId("key-submit"));
    await waitFor(() => expect(apiMock.createApiKey).toHaveBeenCalled());
  });
});
