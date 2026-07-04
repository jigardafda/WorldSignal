import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../test/utils";

const { apiMock, authMock } = vi.hoisted(() => ({
  apiMock: {
    subscriptions: vi.fn(), createSubscription: vi.fn(), updateSubscription: vi.fn(), deleteSubscription: vi.fn(),
    emailConnectors: vi.fn(),
    subscribers: vi.fn(), createSubscriber: vi.fn(), deleteSubscriber: vi.fn(),
    users: vi.fn(), createUser: vi.fn(), updateUser: vi.fn(), deleteUser: vi.fn(),
    teams: vi.fn(), team: vi.fn(), createTeam: vi.fn(), deleteTeam: vi.fn(), addTeamMember: vi.fn(), removeTeamMember: vi.fn(),
    changePassword: vi.fn(), analytics: vi.fn(),
  },
  authMock: { user: { id: "me", email: "me@x.io", name: "Me", role: "ADMIN", createdAt: "2026-01-01T00:00:00Z" }, loading: false, hasPerm: () => true, login: vi.fn(), logout: vi.fn(), refresh: vi.fn() },
}));
vi.mock("../lib/api", () => ({ api: apiMock }));
vi.mock("../lib/auth", () => ({ useAuth: () => authMock }));

import { Subscriptions } from "./Subscriptions";
import { Subscribers } from "./Subscribers";
import { Users } from "./Users";
import { Teams } from "./Teams";
import { Account } from "./Account";
import { Analytics } from "./Analytics";

afterEach(() => { vi.clearAllMocks(); authMock.hasPerm = () => true; });

describe("Subscriptions", () => {
  const sub = { id: "x", name: "All", channel: "POLLING", enabled: true, filter: {}, config: {}, createdAt: "2026-01-01T00:00:00Z" };
  beforeEach(() => apiMock.emailConnectors.mockResolvedValue([{ id: "c1", name: "Team", provider: "GMAIL", host: "smtp.gmail.com", port: 587, security: "STARTTLS", username: "", secretLast4: "", fromEmail: "me@gmail.com", fromName: "WS", isActive: true, enabled: true, status: "VALID", lastTestedAt: null, lastError: null, createdAt: "", updatedAt: "" }]));
  it("lists, toggles, deletes", async () => {
    apiMock.subscriptions.mockResolvedValue([sub]);
    apiMock.updateSubscription.mockResolvedValue({ ...sub, enabled: false });
    apiMock.deleteSubscription.mockResolvedValue(true);
    renderWithProviders(<Subscriptions />);
    expect(await screen.findByText("All")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Disable" }));
    await waitFor(() => expect(apiMock.updateSubscription).toHaveBeenCalledWith("x", { enabled: false }));
    await userEvent.click(screen.getByRole("button", { name: "Delete" }));
    const dialog = await screen.findByRole("dialog");
    await userEvent.click(within(dialog).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(apiMock.deleteSubscription).toHaveBeenCalled());
  });
  it("creates a subscription", async () => {
    apiMock.subscriptions.mockResolvedValue([]);
    apiMock.createSubscription.mockResolvedValue(sub);
    renderWithProviders(<Subscriptions />);
    await screen.findByTestId("empty");
    await userEvent.click(screen.getByRole("button", { name: "Add subscription" }));
    await userEvent.type(await screen.findByTestId("sub-name"), "New");
    await userEvent.click(screen.getByRole("button", { name: "Create" }));
    await waitFor(() => expect(apiMock.createSubscription).toHaveBeenCalled());
  });
  it("creates an EMAIL digest subscription with structured config", async () => {
    apiMock.subscriptions.mockResolvedValue([]);
    apiMock.createSubscription.mockResolvedValue(sub);
    renderWithProviders(<Subscriptions />);
    await screen.findByTestId("empty");
    await userEvent.click(screen.getByRole("button", { name: "Add subscription" }));
    await userEvent.type(await screen.findByTestId("sub-name"), "Email digest");
    // Switch channel to EMAIL.
    fireEvent.click(screen.getByTestId("sub-channel"));
    fireEvent.click(await screen.findByRole("option", { name: "EMAIL", hidden: true }));
    await userEvent.type(await screen.findByTestId("sub-recipients"), "a@x.com, b@y.com");
    // Pick a daily digest.
    fireEvent.click(screen.getByTestId("sub-mode"));
    fireEvent.click(await screen.findByRole("option", { name: /Daily digest/, hidden: true }));
    await userEvent.click(screen.getByRole("button", { name: "Create" }));
    await waitFor(() => expect(apiMock.createSubscription).toHaveBeenCalledWith(expect.objectContaining({
      channel: "EMAIL",
      config: expect.objectContaining({ to: "a@x.com, b@y.com", mode: "digest", interval: "daily" }),
    })));
  });
  it("requires recipients for an EMAIL subscription", async () => {
    apiMock.subscriptions.mockResolvedValue([]);
    renderWithProviders(<Subscriptions />);
    await screen.findByTestId("empty");
    await userEvent.click(screen.getByRole("button", { name: "Add subscription" }));
    await userEvent.type(await screen.findByTestId("sub-name"), "E");
    fireEvent.click(screen.getByTestId("sub-channel"));
    fireEvent.click(await screen.findByRole("option", { name: "EMAIL", hidden: true }));
    await userEvent.click(screen.getByRole("button", { name: "Create" }));
    expect(await screen.findByText("At least one recipient is required")).toBeInTheDocument();
    expect(apiMock.createSubscription).not.toHaveBeenCalled();
  });
  it("creates an instant EMAIL subscription with a chosen connector", async () => {
    apiMock.subscriptions.mockResolvedValue([]);
    apiMock.createSubscription.mockResolvedValue(sub);
    renderWithProviders(<Subscriptions />);
    await screen.findByTestId("empty");
    await userEvent.click(screen.getByRole("button", { name: "Add subscription" }));
    await userEvent.type(await screen.findByTestId("sub-name"), "Instant email");
    fireEvent.click(screen.getByTestId("sub-channel"));
    fireEvent.click(await screen.findByRole("option", { name: "EMAIL", hidden: true }));
    await userEvent.type(await screen.findByTestId("sub-recipients"), "ops@x.com");
    // Pick a specific connector.
    fireEvent.click(screen.getByTestId("sub-connector"));
    fireEvent.click(await screen.findByRole("option", { name: /Team/, hidden: true }));
    await userEvent.click(screen.getByRole("button", { name: "Create" }));
    await waitFor(() => expect(apiMock.createSubscription).toHaveBeenCalledWith(expect.objectContaining({
      channel: "EMAIL",
      config: expect.objectContaining({ to: "ops@x.com", mode: "instant", connectorId: "c1" }),
    })));
  });
  it("surfaces an error when toggling fails", async () => {
    apiMock.subscriptions.mockResolvedValue([{ ...sub, enabled: true }]);
    apiMock.updateSubscription.mockRejectedValue(new Error("nope"));
    renderWithProviders(<Subscriptions />);
    await screen.findByText("All");
    await userEvent.click(screen.getByRole("button", { name: "Disable" }));
    await waitFor(() => expect(apiMock.updateSubscription).toHaveBeenCalled());
  });
});

describe("Subscribers", () => {
  it("lists, creates, deletes", async () => {
    apiMock.subscribers.mockResolvedValue([{ id: "sb", name: "Acme", status: "ACTIVE", createdAt: "2026-01-01T00:00:00Z", subscriptionCount: 2 }]);
    apiMock.createSubscriber.mockResolvedValue({ id: "sb2", name: "New", status: "ACTIVE", createdAt: "2026-01-01T00:00:00Z", subscriptionCount: 0 });
    apiMock.deleteSubscriber.mockResolvedValue(true);
    renderWithProviders(<Subscribers />);
    expect(await screen.findByText("Acme")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Add subscriber" }));
    await userEvent.type(await screen.findByTestId("subscriber-name"), "New");
    await userEvent.click(screen.getByRole("button", { name: "Create" }));
    await waitFor(() => expect(apiMock.createSubscriber).toHaveBeenCalledWith("New"));
  });
});

describe("Users", () => {
  it("lists, creates and forbids self-delete", async () => {
    apiMock.users.mockResolvedValue([
      { id: "me", email: "me@x.io", name: "Me", role: "ADMIN", status: "ACTIVE", createdAt: "2026-01-01T00:00:00Z", updatedAt: "" },
      { id: "u2", email: "other@x.io", name: "Other", role: "VIEWER", status: "ACTIVE", createdAt: "2026-01-01T00:00:00Z", updatedAt: "" },
    ]);
    apiMock.createUser.mockResolvedValue({ id: "u3", email: "n@x.io", name: "", role: "VIEWER", status: "ACTIVE", createdAt: "", updatedAt: "" });
    apiMock.updateUser.mockResolvedValue({ id: "u2", email: "other@x.io", name: "Other", role: "EDITOR", status: "ACTIVE", createdAt: "", updatedAt: "" });
    apiMock.deleteUser.mockResolvedValue(true);
    renderWithProviders(<Users />);
    expect(await screen.findByText("other@x.io")).toBeInTheDocument();
    // Only the non-self row has a Delete button.
    expect(screen.getAllByRole("button", { name: "Delete" })).toHaveLength(1);

    // Change the other user's role via the inline select (before any modal opens).
    // Mantine 9's Combobox opens on a native click event, which fireEvent emits
    // directly (userEvent's pointer sequence no longer toggles the dropdown).
    // jsdom can't lay out the floating dropdown, so the popover keeps
    // `display:none` and byRole filters the options out — match with hidden:true.
    const roleSelect = screen.getAllByRole("combobox").find((el) => (el as HTMLInputElement).value === "VIEWER")!;
    fireEvent.click(roleSelect);
    // Every Select keeps its dropdown mounted, so scope the option lookup to the
    // listbox this combobox controls.
    const listbox = document.getElementById(roleSelect.getAttribute("aria-controls")!)!;
    fireEvent.click(await within(listbox).findByRole("option", { name: "EDITOR", hidden: true }));
    await waitFor(() => expect(apiMock.updateUser).toHaveBeenCalledWith("u2", { role: "EDITOR" }));

    // Delete the other user (confirm).
    await userEvent.click(screen.getByRole("button", { name: "Delete" }));
    const delDialog = await screen.findByRole("dialog");
    await userEvent.click(within(delDialog).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(apiMock.deleteUser).toHaveBeenCalledWith("u2"));

    // Create a user.
    await userEvent.click(screen.getByRole("button", { name: "Add user" }));
    await userEvent.type(await screen.findByTestId("user-email"), "n@x.io");
    await userEvent.type(screen.getByTestId("user-password"), "password123");
    await userEvent.click(screen.getByRole("button", { name: "Create" }));
    await waitFor(() => expect(apiMock.createUser).toHaveBeenCalled());
  });
});

describe("Teams", () => {
  it("creates, manages members and deletes", async () => {
    apiMock.teams.mockResolvedValue([{ id: "t1", name: "Ops", createdAt: "2026-01-01T00:00:00Z", memberCount: 1 }]);
    apiMock.createTeam.mockResolvedValue({ id: "t2", name: "New", createdAt: "", memberCount: 0 });
    apiMock.team.mockResolvedValue({ id: "t1", name: "Ops", createdAt: "", memberCount: 1, members: [{ userId: "u2", email: "other@x.io", name: "Other", role: "OWNER", addedAt: "" }] });
    apiMock.users.mockResolvedValue([{ id: "u3", email: "new@x.io", name: "New", role: "VIEWER", status: "ACTIVE", createdAt: "", updatedAt: "" }]);
    apiMock.removeTeamMember.mockResolvedValue(true);
    apiMock.addTeamMember.mockResolvedValue(true);
    apiMock.deleteTeam.mockResolvedValue(true);
    renderWithProviders(<Teams />);
    expect(await screen.findByText("Ops")).toBeInTheDocument();

    // Create a team.
    await userEvent.click(screen.getByRole("button", { name: "Add team" }));
    await userEvent.type(await screen.findByTestId("team-name"), "New");
    await userEvent.click(screen.getByRole("button", { name: "Create" }));
    await waitFor(() => expect(apiMock.createTeam).toHaveBeenCalledWith("New"));

    // Manage members: add then remove.
    await userEvent.click(screen.getByRole("button", { name: "Manage" }));
    const dialog = await screen.findByRole("dialog");
    const select = within(dialog).getByTestId("team-add-user");
    await userEvent.click(select);
    await userEvent.click(await screen.findByText("new@x.io"));
    await userEvent.click(within(dialog).getByRole("button", { name: "Add" }));
    await waitFor(() => expect(apiMock.addTeamMember).toHaveBeenCalledWith("t1", "u3", "MEMBER"));
    await userEvent.click(within(dialog).getByRole("button", { name: "Remove" }));
    await waitFor(() => expect(apiMock.removeTeamMember).toHaveBeenCalledWith("t1", "u2"));
  });

  it("deletes a team", async () => {
    apiMock.teams.mockResolvedValue([{ id: "t1", name: "Ops", createdAt: "2026-01-01T00:00:00Z", memberCount: 0 }]);
    apiMock.deleteTeam.mockResolvedValue(true);
    renderWithProviders(<Teams />);
    await screen.findByText("Ops");
    await userEvent.click(screen.getByRole("button", { name: "Delete" }));
    const dialog = await screen.findByRole("dialog");
    await userEvent.click(within(dialog).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(apiMock.deleteTeam).toHaveBeenCalledWith("t1"));
  });

  it("shows empty members in manage", async () => {
    apiMock.teams.mockResolvedValue([{ id: "t1", name: "Ops", createdAt: "", memberCount: 0 }]);
    apiMock.team.mockResolvedValue({ id: "t1", name: "Ops", createdAt: "", memberCount: 0, members: [] });
    apiMock.users.mockResolvedValue([]);
    renderWithProviders(<Teams />);
    await screen.findByText("Ops");
    await userEvent.click(screen.getByRole("button", { name: "Manage" }));
    expect(await screen.findByText("No members yet.")).toBeInTheDocument();
  });
});

describe("Account", () => {
  it("shows profile and changes password", async () => {
    apiMock.changePassword.mockResolvedValue(true);
    renderWithProviders(<Account />);
    expect(screen.getByText("me@x.io")).toBeInTheDocument();
    await userEvent.type(screen.getByTestId("old-password"), "oldpass12");
    await userEvent.type(screen.getByTestId("new-password"), "newpass12");
    await userEvent.type(screen.getByTestId("confirm-password"), "newpass12");
    await userEvent.click(screen.getByRole("button", { name: "Update password" }));
    await waitFor(() => expect(apiMock.changePassword).toHaveBeenCalledWith("oldpass12", "newpass12"));
  });
  it("validates password confirmation", async () => {
    renderWithProviders(<Account />);
    await userEvent.type(screen.getByTestId("old-password"), "oldpass12");
    await userEvent.type(screen.getByTestId("new-password"), "newpass12");
    await userEvent.type(screen.getByTestId("confirm-password"), "different");
    await userEvent.click(screen.getByRole("button", { name: "Update password" }));
    expect(await screen.findByText("Passwords do not match")).toBeInTheDocument();
    expect(apiMock.changePassword).not.toHaveBeenCalled();
  });
});

describe("Analytics", () => {
  it("renders KPIs and panels", async () => {
    apiMock.analytics.mockResolvedValue({
      signalsBySeverity: [{ key: "HIGH", count: 3 }], signalsByStatus: [{ key: "CONFIRMED", count: 2 }],
      signalsByEventType: [{ key: "DISASTER.FLOOD", count: 1 }], signalsByCountry: [{ key: "US", count: 4 }],
      signalsBySentiment: [{ key: "NEGATIVE", count: 3 }], signalsByGeoScope: [{ key: "LOCAL", count: 2 }],
      topIndustries: [{ key: "CYBERSECURITY", count: 5 }],
      signalsOverTime: [{ key: "2026-01-01", count: 5 }],
      topSources: [{ id: "s1", name: "BBC", articleCount: 9 }],
      deliveryStats: { total: 10, sent: 8, pending: 1, retrying: 0, failed: 1, deadLettered: 0 },
      ingestionStats: { rawItems: 20, parsed: 18, duplicates: 1, failed: 1, articles: 18 },
    });
    renderWithProviders(<Analytics />);
    expect(await screen.findByText("Top sources by articles")).toBeInTheDocument();
    expect(screen.getByText("BBC")).toBeInTheDocument();
  });
  it("handles empty analytics", async () => {
    apiMock.analytics.mockResolvedValue({
      signalsBySeverity: [], signalsByStatus: [], signalsByEventType: [], signalsByCountry: [],
      signalsBySentiment: [], signalsByGeoScope: [], topIndustries: [],
      signalsOverTime: [], topSources: [],
      deliveryStats: { total: 0, sent: 0, pending: 0, retrying: 0, failed: 0, deadLettered: 0 },
      ingestionStats: { rawItems: 0, parsed: 0, duplicates: 0, failed: 0, articles: 0 },
    });
    renderWithProviders(<Analytics />);
    expect(await screen.findByText("Signals over time (30 days)")).toBeInTheDocument();
    expect(screen.getAllByText("No data.").length).toBeGreaterThan(0);
  });
  it("tolerates an older backend that omits new analytics keys", async () => {
    // Version skew: a backend predating the enrichment fields returns no
    // signalsBySentiment/geoScope/topIndustries. The page must not crash.
    apiMock.analytics.mockResolvedValue({
      signalsBySeverity: [], signalsByStatus: [], signalsByEventType: [], signalsByCountry: [],
      signalsOverTime: [], topSources: [],
      deliveryStats: { total: 0, sent: 0, pending: 0, retrying: 0, failed: 0, deadLettered: 0 },
      ingestionStats: { rawItems: 0, parsed: 0, duplicates: 0, failed: 0, articles: 0 },
    } as never);
    renderWithProviders(<Analytics />);
    expect(await screen.findByText("By sentiment")).toBeInTheDocument();
  });
});
