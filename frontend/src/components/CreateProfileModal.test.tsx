import { describe, expect, it, vi, beforeEach } from "vitest";
import { screen, waitFor, fireEvent } from "@testing-library/react";
import { renderWithProviders } from "../test/utils";

const { apiMock } = vi.hoisted(() => ({
  apiMock: {
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

import { CreateProfileModal } from "./CreateProfileModal";

beforeEach(() => {
  vi.clearAllMocks();
  apiMock.createSubscription.mockResolvedValue({ id: "new1", name: "New profile" });
  apiMock.setSubscriptionInterests.mockResolvedValue({ ok: true });
});

describe("CreateProfileModal", () => {
  it("creates a blank profile on a SINGLE click (no double-click)", async () => {
    const onCreated = vi.fn();
    renderWithProviders(<CreateProfileModal opened onClose={() => {}} onCreated={onCreated} />);

    fireEvent.click(screen.getByText("Start blank instead"));

    await waitFor(() => expect(apiMock.createSubscription).toHaveBeenCalledTimes(1));
    expect(apiMock.createSubscription).toHaveBeenCalledWith({ name: "New profile", channel: "POLLING" });
    await waitFor(() => expect(onCreated).toHaveBeenCalledWith("new1"));
  });

  it("uses the typed name for a blank profile", async () => {
    renderWithProviders(<CreateProfileModal opened onClose={() => {}} onCreated={() => {}} />);
    fireEvent.change(screen.getByTestId("name-input"), { target: { value: "Nike risk" } });
    fireEvent.click(screen.getByText("Start blank instead"));
    await waitFor(() =>
      expect(apiMock.createSubscription).toHaveBeenCalledWith({ name: "Nike risk", channel: "POLLING" }),
    );
  });

  it("drafts a profile from a document and creates it with its interests", async () => {
    apiMock.draftProfileFromDocument.mockResolvedValue({
      name: "Adidas watch",
      summary: "Drafted.",
      minScore: 6.5,
      minSeverity: "MEDIUM",
      source: "llm",
      interests: { "entity:Adidas": 5, "tag:SPORTS": 4 },
      reasons: [{ key: "entity:Adidas", why: "the brand", origin: "doc" }],
    });
    renderWithProviders(<CreateProfileModal opened onClose={() => {}} onCreated={() => {}} />);

    fireEvent.change(screen.getByTestId("doc-input"), {
      target: { value: "Adidas media kit — sponsors athletes across running and football." },
    });
    fireEvent.click(screen.getByText("Analyze document"));

    await waitFor(() => expect(screen.getByText("Here's what we built")).toBeInTheDocument());
    expect(screen.getByText("Adidas")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Create profile"));
    await waitFor(() =>
      expect(apiMock.setSubscriptionInterests).toHaveBeenCalledWith("new1", { "entity:Adidas": 5, "tag:SPORTS": 4 }),
    );
  });
});
