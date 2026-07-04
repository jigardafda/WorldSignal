import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../test/utils";

const { apiMock } = vi.hoisted(() => ({
  apiMock: { entities: vi.fn(), attributeDictionary: vi.fn() },
}));
vi.mock("../lib/api", () => ({ api: apiMock }));

const navigateMock = vi.hoisted(() => vi.fn());
vi.mock("react-router-dom", async (orig) => ({
  ...(await orig<typeof import("react-router-dom")>()),
  useNavigate: () => navigateMock,
}));

import { Entities } from "./Entities";

const rows = [
  { name: "Red Cross", type: "ORG", signalCount: 5 },
  { name: "Coastal City", type: "LOCATION", signalCount: 2 },
];

afterEach(() => vi.clearAllMocks());
beforeEach(() => {
  apiMock.attributeDictionary.mockResolvedValue([
    { key: "entityType", label: "Entity Type", kind: "ENUM", description: "", values: [
      { code: "ORG", label: "Organization" }, { code: "LOCATION", label: "Location" },
    ] },
  ]);
});

describe("Entities", () => {
  it("lists entities with type and counts", async () => {
    apiMock.entities.mockResolvedValue(rows);
    renderWithProviders(<Entities />);
    expect(await screen.findByText("Red Cross")).toBeInTheDocument();
    expect(screen.getByText("Coastal City")).toBeInTheDocument();
    expect(screen.getByText("5")).toBeInTheDocument();
  });

  it("searches by name", async () => {
    apiMock.entities.mockResolvedValue(rows);
    renderWithProviders(<Entities />);
    await screen.findByText("Red Cross");
    await userEvent.type(screen.getByTestId("entity-search"), "red{Enter}");
    await waitFor(() => expect(apiMock.entities).toHaveBeenCalledWith(expect.objectContaining({ search: "red" }), 200));
  });

  it("filters by type", async () => {
    apiMock.entities.mockResolvedValue(rows);
    renderWithProviders(<Entities />);
    await screen.findByText("Red Cross");
    fireEvent.click(screen.getByTestId("entity-type"));
    const listbox = document.getElementById(screen.getByTestId("entity-type").getAttribute("aria-controls")!)!;
    fireEvent.click(await within(listbox).findByRole("option", { name: "Organization", hidden: true }));
    await waitFor(() => expect(apiMock.entities).toHaveBeenCalledWith(expect.objectContaining({ type: "ORG" }), 200));
  });

  it("drills into signals for an entity", async () => {
    apiMock.entities.mockResolvedValue(rows);
    renderWithProviders(<Entities />);
    await userEvent.click(await screen.findByText("Red Cross"));
    expect(navigateMock).toHaveBeenCalledWith("/signals?entity=Red%20Cross");
  });

  it("shows the empty state", async () => {
    apiMock.entities.mockResolvedValue([]);
    renderWithProviders(<Entities />);
    expect(await screen.findByTestId("empty")).toBeInTheDocument();
  });
});
