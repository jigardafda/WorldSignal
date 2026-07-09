import { describe, expect, it, vi, beforeEach } from "vitest";
import { screen, waitFor, fireEvent, within } from "@testing-library/react";
import { renderWithProviders } from "../test/utils";

const { apiMock } = vi.hoisted(() => ({
  apiMock: {
    subscriptions: vi.fn(),
    subscriptionFeed: vi.fn(),
    subscriptionInterests: vi.fn(),
    recordSignalFeedback: vi.fn(),
    setSubscriptionInterests: vi.fn(),
    draftProfileFromDocument: vi.fn(),
    createSubscription: vi.fn(),
    updateSubscription: vi.fn(),
    deleteSubscription: vi.fn(),
  },
}));
vi.mock("../lib/api", () => ({ api: apiMock }));
vi.mock("@mantine/notifications", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@mantine/notifications")>();
  return { ...actual, notifications: { show: vi.fn() } };
});

import { ForYou } from "./ForYou";

const FEED = [
  {
    id: "sg1", title: "Newer, lower score", summary: "s", eventType: "DISASTER.EARTHQUAKE",
    country: "IN", region: "", sentiment: "NEGATIVE", influence: "HIGH", severity: "HIGH",
    ageHours: 1, score: 8.0, reasons: ["tag:DISASTER"],
  },
  {
    id: "sg2", title: "Older, higher score", summary: "s", eventType: "DISASTER.FLOOD",
    country: "CN", region: "", sentiment: "NEGATIVE", influence: "HIGH", severity: "HIGH",
    ageHours: 30, score: 9.5, reasons: ["tag:DISASTER", "country:CN"],
  },
];
const PROFILE = [{ id: "p1", name: "Sponsorship risk", channel: "POLLING", enabled: true, filter: {}, config: {}, createdAt: "2026-07-01T00:00:00Z" }];

beforeEach(() => {
  vi.clearAllMocks();
  apiMock.subscriptions.mockResolvedValue(PROFILE);
  apiMock.subscriptionFeed.mockResolvedValue(FEED);
  apiMock.subscriptionInterests.mockResolvedValue({ "tag:DISASTER": 5 });
  apiMock.recordSignalFeedback.mockResolvedValue(true);
  apiMock.setSubscriptionInterests.mockResolvedValue({ ok: true });
  apiMock.updateSubscription.mockResolvedValue({ id: "p1", name: "Renamed" });
  apiMock.deleteSubscription.mockResolvedValue(true);
});

async function ready() {
  await waitFor(() => expect(screen.getByText("Older, higher score")).toBeInTheDocument());
}

describe("ForYou", () => {
  it("renders a ranked feed with score and reasons for the active profile", async () => {
    renderWithProviders(<ForYou />);
    await ready();
    expect(screen.getByText("9.5")).toBeInTheDocument();
    expect(apiMock.subscriptionFeed).toHaveBeenCalledWith("p1", 2, 40);
  });

  it("summarizes what the profile ranks by, above the feed", async () => {
    apiMock.subscriptionInterests.mockResolvedValue({ "tag:DISASTER": 5, "sentiment:NEGATIVE": 2 });
    renderWithProviders(<ForYou />);
    const summary = await screen.findByTestId("interest-summary");
    expect(within(summary).getByText(/Ranking by/i)).toBeInTheDocument();
    expect(within(summary).getByText("Disaster")).toBeInTheDocument();
    expect(within(summary).getByText("×5")).toBeInTheDocument();
  });

  it("re-sorts by recency, and filters by strength", async () => {
    renderWithProviders(<ForYou />);
    await ready();
    // Default relevance: sg2 (9.5) before sg1 (8.0).
    let cards = screen.getAllByTestId("feed-card");
    expect(within(cards[0]).getByText("Older, higher score")).toBeInTheDocument();
    // Recency: sg1 (1h) before sg2 (30h).
    fireEvent.click(screen.getByText("Recency"));
    cards = screen.getAllByTestId("feed-card");
    expect(within(cards[0]).getByText("Newer, lower score")).toBeInTheDocument();
    // "All" re-requests the feed with no cutoff.
    fireEvent.click(screen.getByText("All"));
    await waitFor(() => expect(apiMock.subscriptionFeed).toHaveBeenCalledWith("p1", 0, 40));
  });

  it("records UP, DOWN and OPEN feedback", async () => {
    renderWithProviders(<ForYou />);
    await ready();
    fireEvent.click(screen.getAllByLabelText("More like this")[0]);
    expect(apiMock.recordSignalFeedback).toHaveBeenCalledWith("p1", "sg2", "UP");
    fireEvent.click(screen.getAllByLabelText("Less like this")[0]);
    expect(apiMock.recordSignalFeedback).toHaveBeenCalledWith("p1", "sg2", "DOWN");
    fireEvent.click(screen.getAllByText("Open")[0]);
    expect(apiMock.recordSignalFeedback).toHaveBeenCalledWith("p1", "sg2", "OPEN");
  });

  it("opens the interest editor and saves", async () => {
    renderWithProviders(<ForYou />);
    await ready();
    fireEvent.click(screen.getByText("Edit interests"));
    await waitFor(() => expect(screen.getByText(/watches/)).toBeInTheDocument());
    fireEvent.click(screen.getByText("Save interests"));
    await waitFor(() => expect(apiMock.setSubscriptionInterests).toHaveBeenCalledWith("p1", { "tag:DISASTER": 5 }));
  });

  it("renames the active profile from the actions menu", async () => {
    renderWithProviders(<ForYou />);
    await ready();
    fireEvent.click(screen.getByLabelText("Profile actions"));
    fireEvent.click(await screen.findByText("Rename profile"));
    const input = await screen.findByLabelText("Profile name");
    fireEvent.change(input, { target: { value: "New name" } });
    fireEvent.click(screen.getByText("Save"));
    await waitFor(() => expect(apiMock.updateSubscription).toHaveBeenCalledWith("p1", { name: "New name" }));
  });

  it("deletes the active profile from the actions menu", async () => {
    renderWithProviders(<ForYou />);
    await ready();
    fireEvent.click(screen.getByLabelText("Profile actions"));
    fireEvent.click(await screen.findByText("Delete profile"));
    fireEvent.click(await screen.findByRole("button", { name: "Delete profile" }));
    await waitFor(() => expect(apiMock.deleteSubscription).toHaveBeenCalledWith("p1"));
  });

  it("creates a profile from the create modal and selects it", async () => {
    apiMock.createSubscription.mockResolvedValue({ id: "new1", name: "New profile" });
    renderWithProviders(<ForYou />);
    await ready();
    fireEvent.click(screen.getByText("New profile"));
    await screen.findByTestId("create-profile");
    fireEvent.click(await screen.findByText("Start blank instead"));
    await waitFor(() => expect(apiMock.createSubscription).toHaveBeenCalled());
    // onCreated reloads the profile list.
    await waitFor(() => expect(apiMock.subscriptions.mock.calls.length).toBeGreaterThan(1));
  });

  it("shows an empty state when nothing clears the relevance bar", async () => {
    apiMock.subscriptionFeed.mockResolvedValue([]);
    renderWithProviders(<ForYou />);
    await waitFor(() => expect(screen.getByText(/No recent signals clear the bar/)).toBeInTheDocument());
  });

  it("invites the user to create a profile when there are none", async () => {
    apiMock.subscriptions.mockResolvedValue([]);
    renderWithProviders(<ForYou />);
    const cta = await screen.findByText(/Create your first profile/);
    expect(apiMock.subscriptionFeed).not.toHaveBeenCalled();
    fireEvent.click(cta);
    expect(await screen.findByTestId("create-profile")).toBeInTheDocument();
  });
});
