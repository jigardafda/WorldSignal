import { describe, expect, it, vi, beforeEach } from "vitest";
import { screen, waitFor, fireEvent } from "@testing-library/react";
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
    id: "sg1",
    title: "Big earthquake strikes the coast",
    summary: "A quake struck.",
    eventType: "DISASTER.EARTHQUAKE",
    country: "IN",
    region: "",
    sentiment: "NEGATIVE",
    influence: "HIGH",
    severity: "HIGH",
    ageHours: 2,
    score: 9.2,
    reasons: ["tag:DISASTER", "country:IN"],
  },
];

beforeEach(() => {
  vi.clearAllMocks();
  apiMock.subscriptionInterests.mockResolvedValue({});
  apiMock.recordSignalFeedback.mockResolvedValue(true);
});

describe("ForYou", () => {
  it("renders a ranked feed with score and reasons for the active profile", async () => {
    apiMock.subscriptions.mockResolvedValue([{ id: "p1", name: "Sponsorship risk", channel: "POLLING", enabled: true }]);
    apiMock.subscriptionFeed.mockResolvedValue(FEED);

    renderWithProviders(<ForYou />);

    await waitFor(() => expect(screen.getByText("Big earthquake strikes the coast")).toBeInTheDocument());
    // relevance score ring shows the score
    expect(screen.getByText("9.2")).toBeInTheDocument();
    // the "why it ranked" reason is humanized (DISASTER -> Disaster)
    expect(screen.getByText("Disaster")).toBeInTheDocument();
    // the feed was requested for the active profile
    expect(apiMock.subscriptionFeed).toHaveBeenCalledWith("p1", 0, 40);
  });

  it("sends feedback when a signal is voted on", async () => {
    apiMock.subscriptions.mockResolvedValue([{ id: "p1", name: "Sponsorship risk", channel: "POLLING", enabled: true }]);
    apiMock.subscriptionFeed.mockResolvedValue(FEED);

    renderWithProviders(<ForYou />);
    await waitFor(() => expect(screen.getByText("Big earthquake strikes the coast")).toBeInTheDocument());

    fireEvent.click(screen.getByLabelText("More like this"));
    expect(apiMock.recordSignalFeedback).toHaveBeenCalledWith("p1", "sg1", "UP");
  });

  it("invites the user to create a profile when there are none", async () => {
    apiMock.subscriptions.mockResolvedValue([]);
    renderWithProviders(<ForYou />);
    await waitFor(() => expect(screen.getByText(/Create your first profile/)).toBeInTheDocument());
    expect(apiMock.subscriptionFeed).not.toHaveBeenCalled();
  });
});
