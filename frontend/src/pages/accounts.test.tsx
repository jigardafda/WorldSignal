import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../test/utils";

const { apiMock, authMock } = vi.hoisted(() => ({
  apiMock: {
    accounts: vi.fn(),
    createAccount: vi.fn(),
    updateAccount: vi.fn(),
  },
  authMock: { user: { id: "me", email: "a@b.c", role: "ADMIN" }, loading: false, hasPerm: (_p: string): boolean => true, login: vi.fn(), logout: vi.fn(), refresh: vi.fn() },
}));
vi.mock("../lib/api", () => ({ api: apiMock }));
vi.mock("../lib/auth", () => ({ useAuth: () => authMock }));
vi.mock("@mantine/notifications", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@mantine/notifications")>();
  return { ...actual, notifications: { show: vi.fn() } };
});

import { Accounts } from "./Accounts";

const account = (o = {}) => ({ id: "a1", name: "Acme Corp", slug: "acme-corp", status: "ACTIVE", plan: "PRO", createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-01-01T00:00:00Z", ...o });

afterEach(() => { vi.clearAllMocks(); authMock.hasPerm = () => true; });
beforeEach(() => { apiMock.accounts.mockResolvedValue([account()]); });

describe("Accounts", () => {
  it("lists accounts with slug, plan and status", async () => {
    renderWithProviders(<Accounts />);
    expect(await screen.findByText("Acme Corp")).toBeInTheDocument();
    expect(screen.getByText("acme-corp")).toBeInTheDocument();
    expect(screen.getByText("PRO")).toBeInTheDocument();
  });

  it("shows the empty state", async () => {
    apiMock.accounts.mockResolvedValue([]);
    renderWithProviders(<Accounts />);
    expect(await screen.findByTestId("empty")).toBeInTheDocument();
  });

  it("creates an account through the modal", async () => {
    apiMock.createAccount.mockResolvedValue(account({ id: "a2" }));
    renderWithProviders(<Accounts />);
    await screen.findByText("Acme Corp");
    await userEvent.click(screen.getByRole("button", { name: "Add account" }));
    await userEvent.type(await screen.findByTestId("account-name"), "Globex");
    await userEvent.type(screen.getByTestId("account-slug"), "globex");
    await userEvent.click(screen.getByRole("button", { name: "Create" }));
    await waitFor(() => expect(apiMock.createAccount).toHaveBeenCalledWith({ name: "Globex", slug: "globex", plan: "FREE" }));
  });

  it("omits an empty slug on create", async () => {
    apiMock.createAccount.mockResolvedValue(account({ id: "a3" }));
    renderWithProviders(<Accounts />);
    await screen.findByText("Acme Corp");
    await userEvent.click(screen.getByRole("button", { name: "Add account" }));
    await userEvent.type(await screen.findByTestId("account-name"), "Initech");
    await userEvent.click(screen.getByRole("button", { name: "Create" }));
    await waitFor(() => expect(apiMock.createAccount).toHaveBeenCalledWith({ name: "Initech", slug: undefined, plan: "FREE" }));
  });

  it("validates the required name", async () => {
    renderWithProviders(<Accounts />);
    await screen.findByText("Acme Corp");
    await userEvent.click(screen.getByRole("button", { name: "Add account" }));
    // Whitespace passes the browser's native `required` check but trips the
    // form-level validator (v.trim()), so we exercise the validation branch.
    await userEvent.type(await screen.findByTestId("account-name"), "   ");
    await userEvent.click(screen.getByRole("button", { name: "Create" }));
    expect(await screen.findByText("Name is required")).toBeInTheDocument();
    expect(apiMock.createAccount).not.toHaveBeenCalled();
  });

  it("surfaces a create error", async () => {
    apiMock.createAccount.mockRejectedValue(new Error("slug taken"));
    renderWithProviders(<Accounts />);
    await screen.findByText("Acme Corp");
    await userEvent.click(screen.getByRole("button", { name: "Add account" }));
    await userEvent.type(await screen.findByTestId("account-name"), "Dup");
    await userEvent.click(screen.getByRole("button", { name: "Create" }));
    await waitFor(() => expect(apiMock.createAccount).toHaveBeenCalled());
  });

  it("suspends an active account and reactivates a suspended one", async () => {
    apiMock.updateAccount.mockResolvedValue(account({ status: "SUSPENDED" }));
    renderWithProviders(<Accounts />);
    await screen.findByText("Acme Corp");
    await userEvent.click(screen.getByTestId("toggle-a1"));
    await waitFor(() => expect(apiMock.updateAccount).toHaveBeenCalledWith("a1", { status: "SUSPENDED" }));

    apiMock.accounts.mockResolvedValue([account({ status: "SUSPENDED" })]);
    apiMock.updateAccount.mockResolvedValue(account({ status: "ACTIVE" }));
    renderWithProviders(<Accounts />);
    const activate = await screen.findByRole("button", { name: "Activate" });
    await userEvent.click(activate);
    await waitFor(() => expect(apiMock.updateAccount).toHaveBeenCalledWith("a1", { status: "ACTIVE" }));
  });

  it("surfaces a toggle error", async () => {
    apiMock.updateAccount.mockRejectedValue(new Error("nope"));
    renderWithProviders(<Accounts />);
    await screen.findByText("Acme Corp");
    await userEvent.click(screen.getByTestId("toggle-a1"));
    await waitFor(() => expect(apiMock.updateAccount).toHaveBeenCalled());
  });

  it("hides management actions without the permission", async () => {
    authMock.hasPerm = () => false;
    renderWithProviders(<Accounts />);
    await screen.findByText("Acme Corp");
    expect(screen.queryByRole("button", { name: "Add account" })).toBeNull();
    expect(screen.queryByTestId("toggle-a1")).toBeNull();
  });
});
