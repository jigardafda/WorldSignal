import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../test/utils";

const { apiMock } = vi.hoisted(() => ({
  apiMock: { mySubscriptions: vi.fn(), createMySubscription: vi.fn(), updateMySubscription: vi.fn(), deleteMySubscription: vi.fn() },
}));
vi.mock("../lib/api", () => ({ api: apiMock }));
vi.mock("@mantine/notifications", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@mantine/notifications")>();
  return { ...actual, notifications: { show: vi.fn() } };
});

import { MySubscriptions } from "./MySubscriptions";

const sub = (o = {}) => ({ id: "s1", name: "Alerts", channel: "WEBHOOK", enabled: true, filter: {}, config: {}, createdAt: "2026-01-01T00:00:00Z", ...o });

afterEach(() => vi.clearAllMocks());
beforeEach(() => { apiMock.mySubscriptions.mockResolvedValue([sub()]); });

describe("MySubscriptions (tenant)", () => {
  it("lists the tenant's subscriptions", async () => {
    renderWithProviders(<MySubscriptions />);
    expect(await screen.findByText("Alerts")).toBeInTheDocument();
    expect(screen.getByText("WEBHOOK")).toBeInTheDocument();
  });

  it("shows the empty state", async () => {
    apiMock.mySubscriptions.mockResolvedValue([]);
    renderWithProviders(<MySubscriptions />);
    expect(await screen.findByTestId("empty")).toBeInTheDocument();
  });

  it("creates a subscription with a channel and destination", async () => {
    apiMock.mySubscriptions.mockResolvedValue([]);
    apiMock.createMySubscription.mockResolvedValue(sub({ id: "s2" }));
    renderWithProviders(<MySubscriptions />);
    await screen.findByTestId("empty");
    await userEvent.click(screen.getByTestId("add-subscription"));
    await userEvent.type(await screen.findByTestId("subscription-name"), "Cyber alerts");
    // channel select (Mantine combobox — open + pick, matching hidden option)
    const channel = screen.getByTestId("subscription-channel");
    fireEvent.click(channel);
    const lb = document.getElementById(channel.getAttribute("aria-controls")!)!;
    fireEvent.click(await within(lb).findByRole("option", { name: "EMAIL", hidden: true }));
    await userEvent.type(screen.getByTestId("subscription-url"), "https://hook.example");
    await userEvent.click(screen.getByRole("button", { name: "Create" }));
    await waitFor(() => expect(apiMock.createMySubscription).toHaveBeenCalledWith(expect.objectContaining({ name: "Cyber alerts", channel: "EMAIL", config: { url: "https://hook.example" } })));
  });

  it("validates the required name", async () => {
    apiMock.mySubscriptions.mockResolvedValue([]);
    renderWithProviders(<MySubscriptions />);
    await screen.findByTestId("empty");
    await userEvent.click(screen.getByTestId("add-subscription"));
    await userEvent.type(await screen.findByTestId("subscription-name"), "   ");
    await userEvent.click(screen.getByRole("button", { name: "Create" }));
    expect(await screen.findByText("Name is required")).toBeInTheDocument();
    expect(apiMock.createMySubscription).not.toHaveBeenCalled();
  });

  it("surfaces a create error", async () => {
    apiMock.mySubscriptions.mockResolvedValue([]);
    apiMock.createMySubscription.mockRejectedValue(new Error("nope"));
    renderWithProviders(<MySubscriptions />);
    await screen.findByTestId("empty");
    await userEvent.click(screen.getByTestId("add-subscription"));
    await userEvent.type(await screen.findByTestId("subscription-name"), "X");
    await userEvent.click(screen.getByRole("button", { name: "Create" }));
    await waitFor(() => expect(apiMock.createMySubscription).toHaveBeenCalled());
  });

  it("pauses/resumes and surfaces a toggle error", async () => {
    apiMock.updateMySubscription.mockResolvedValue(sub({ enabled: false }));
    renderWithProviders(<MySubscriptions />);
    await screen.findByText("Alerts");
    await userEvent.click(screen.getByTestId("toggle-s1"));
    await waitFor(() => expect(apiMock.updateMySubscription).toHaveBeenCalledWith("s1", { enabled: false }));

    apiMock.updateMySubscription.mockRejectedValue(new Error("boom"));
    apiMock.mySubscriptions.mockResolvedValue([sub({ enabled: false })]);
    renderWithProviders(<MySubscriptions />);
    const resume = await screen.findByRole("button", { name: "Resume" });
    await userEvent.click(resume);
    await waitFor(() => expect(apiMock.updateMySubscription).toHaveBeenCalledWith("s1", { enabled: true }));
  });

  it("deletes a subscription", async () => {
    apiMock.deleteMySubscription.mockResolvedValue(true);
    renderWithProviders(<MySubscriptions />);
    await screen.findByText("Alerts");
    await userEvent.click(screen.getByRole("button", { name: "Delete" }));
    const dialog = await screen.findByRole("dialog");
    await userEvent.click(within(dialog).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(apiMock.deleteMySubscription).toHaveBeenCalledWith("s1"));
  });
});
