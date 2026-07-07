import { describe, expect, it, vi, beforeEach } from "vitest";
import { screen, waitFor, fireEvent } from "@testing-library/react";
import { renderWithProviders } from "../test/utils";

const { apiMock } = vi.hoisted(() => ({
  apiMock: {
    subscriptions: vi.fn(),
    subscriptionInterests: vi.fn(),
    updateSubscription: vi.fn(),
    deleteSubscription: vi.fn(),
    createSubscription: vi.fn(),
    setSubscriptionInterests: vi.fn(),
    draftProfileFromDocument: vi.fn(),
  },
}));
vi.mock("../lib/api", () => ({ api: apiMock }));
vi.mock("@mantine/notifications", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@mantine/notifications")>();
  return { ...actual, notifications: { show: vi.fn() } };
});

import { Profiles } from "./Profiles";

const PROFILES = [
  { id: "p1", name: "Sponsorship risk", channel: "POLLING", enabled: true, filter: {}, config: {}, createdAt: "2026-07-01T00:00:00Z" },
  { id: "p2", name: "Demand", channel: "POLLING", enabled: true, filter: {}, config: {}, createdAt: "2026-07-02T00:00:00Z" },
];

beforeEach(() => {
  vi.clearAllMocks();
  apiMock.subscriptions.mockResolvedValue(PROFILES);
  apiMock.subscriptionInterests.mockResolvedValue({ "tag:DISASTER": 5, "entity:Nike": 4 });
  apiMock.updateSubscription.mockResolvedValue({ id: "p1", name: "Renamed" });
  apiMock.deleteSubscription.mockResolvedValue(true);
});

describe("Profiles", () => {
  it("lists profiles with their interest counts", async () => {
    renderWithProviders(<Profiles />);
    await waitFor(() => expect(screen.getByText("Sponsorship risk")).toBeInTheDocument());
    expect(screen.getByText("Demand")).toBeInTheDocument();
    await waitFor(() => expect(screen.getAllByText("2 interests").length).toBeGreaterThan(0));
  });

  it("renames a profile", async () => {
    renderWithProviders(<Profiles />);
    await waitFor(() => expect(screen.getByText("Sponsorship risk")).toBeInTheDocument());

    fireEvent.click(screen.getAllByLabelText(/Actions for/)[0]);
    fireEvent.click(await screen.findByText("Rename"));
    const input = await screen.findByLabelText("Profile name");
    fireEvent.change(input, { target: { value: "Nike risk" } });
    fireEvent.click(screen.getByText("Save"));

    await waitFor(() => expect(apiMock.updateSubscription).toHaveBeenCalledWith("p1", { name: "Nike risk" }));
  });

  it("deletes a profile after confirming in the dialog", async () => {
    renderWithProviders(<Profiles />);
    await waitFor(() => expect(screen.getByText("Sponsorship risk")).toBeInTheDocument());

    fireEvent.click(screen.getAllByLabelText(/Actions for/)[0]);
    fireEvent.click(await screen.findByText("Delete")); // menu item opens the confirm dialog
    fireEvent.click(await screen.findByRole("button", { name: "Delete profile" })); // confirm
    await waitFor(() => expect(apiMock.deleteSubscription).toHaveBeenCalledWith("p1"));
  });
});
