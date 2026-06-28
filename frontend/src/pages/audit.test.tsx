import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../test/utils";

const { apiMock } = vi.hoisted(() => ({ apiMock: { auditLogs: vi.fn() } }));
vi.mock("../lib/api", () => ({ api: apiMock }));

import { AuditLog } from "./AuditLog";

const row = (o = {}) => ({ id: "a1", actorId: "u1", actorEmail: "admin@x.io", actorRole: "ADMIN", action: "USER_CREATED", targetType: "user", targetId: "u2", metadata: { role: "EDITOR" }, createdAt: "2026-01-01T00:00:00Z", ...o });

afterEach(() => vi.clearAllMocks());

describe("AuditLog", () => {
  it("lists actions and searches", async () => {
    apiMock.auditLogs.mockResolvedValue({ items: [row()], total: 1 });
    renderWithProviders(<AuditLog />);
    expect(await screen.findByText("USER_CREATED")).toBeInTheDocument();
    expect(screen.getByText("admin@x.io")).toBeInTheDocument();
    expect(screen.getByText("user:u2")).toBeInTheDocument();
    await userEvent.type(screen.getByTestId("audit-search"), "login{Enter}");
    await waitFor(() => expect(apiMock.auditLogs).toHaveBeenCalledWith(expect.objectContaining({ search: "login" }), 50, 0));
  });

  it("renders the empty state", async () => {
    apiMock.auditLogs.mockResolvedValue({ items: [], total: 0 });
    renderWithProviders(<AuditLog />);
    expect(await screen.findByTestId("empty")).toBeInTheDocument();
  });

  it("shows the error state", async () => {
    apiMock.auditLogs.mockRejectedValue(new Error("down"));
    renderWithProviders(<AuditLog />);
    expect(await screen.findByTestId("error")).toBeInTheDocument();
  });

  it("renders rows with missing actor/target/metadata gracefully", async () => {
    apiMock.auditLogs.mockResolvedValue({ items: [row({ actorEmail: null, actorRole: null, targetType: null, targetId: null, metadata: null })], total: 1 });
    renderWithProviders(<AuditLog />);
    expect(await screen.findByText("USER_CREATED")).toBeInTheDocument();
  });
});
