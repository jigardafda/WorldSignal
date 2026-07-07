import { describe, expect, it, vi, beforeEach } from "vitest";
import { screen, waitFor, fireEvent } from "@testing-library/react";
import { renderWithProviders } from "../test/utils";

const { apiMock } = vi.hoisted(() => ({ apiMock: { setSubscriptionInterests: vi.fn() } }));
vi.mock("../lib/api", () => ({ api: apiMock }));
vi.mock("@mantine/notifications", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@mantine/notifications")>();
  return { ...actual, notifications: { show: vi.fn() } };
});

import { InterestGraphDrawer } from "./InterestGraphDrawer";

beforeEach(() => {
  vi.clearAllMocks();
  apiMock.setSubscriptionInterests.mockResolvedValue({ ok: true });
});

describe("InterestGraphDrawer", () => {
  it("adds an entity, removes an existing interest, and saves the result", async () => {
    const onSaved = vi.fn();
    renderWithProviders(
      <InterestGraphDrawer
        opened
        onClose={() => {}}
        subscriptionId="p1"
        profileName="Nike"
        initial={{ "tag:DISASTER": 5 }}
        onSaved={onSaved}
      />,
    );
    // seeds from `initial`
    await waitFor(() => expect(screen.getByText("Disaster")).toBeInTheDocument());

    // add an entity
    fireEvent.change(screen.getByTestId("val"), { target: { value: "Adidas" } });
    fireEvent.click(screen.getByText("Add"));
    expect(screen.getByText("Adidas")).toBeInTheDocument();

    // remove the seeded Disaster interest
    fireEvent.click(screen.getByLabelText("Remove Disaster"));
    await waitFor(() => expect(screen.queryByText("Disaster")).not.toBeInTheDocument());

    fireEvent.click(screen.getByText("Save interests"));
    await waitFor(() =>
      expect(apiMock.setSubscriptionInterests).toHaveBeenCalledWith("p1", { "entity:Adidas": 3 }),
    );
    expect(onSaved).toHaveBeenCalled();
  });

  it("switches the value input to a topic picker for the Topic dimension", async () => {
    renderWithProviders(
      <InterestGraphDrawer opened onClose={() => {}} subscriptionId="p1" profileName="Nike" initial={{}} onSaved={() => {}} />,
    );
    // Default is entity → free-text value input.
    expect(screen.getByTestId("val")).toBeInTheDocument();
    // Choose the Topic dimension → the value becomes a select (no free-text input).
    fireEvent.click(screen.getByTestId("dim"));
    fireEvent.click(await screen.findByText("Topic — a category"));
    await waitFor(() => expect(screen.getByTestId("val-select")).toBeInTheDocument());
    expect(screen.queryByTestId("val")).not.toBeInTheDocument();
  });
});
