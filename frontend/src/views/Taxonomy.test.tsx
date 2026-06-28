import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { Taxonomy } from "./Taxonomy";
import { api } from "../api";

vi.mock("../api", () => ({ api: { taxonomy: vi.fn() } }));

afterEach(() => vi.clearAllMocks());

describe("Taxonomy", () => {
  it("renders domains and leaf chips", async () => {
    (api.taxonomy as any).mockResolvedValue([
      { code: "DISASTER", label: "Disaster", children: [{ code: "DISASTER.FLOOD", label: "Flood" }] },
      { code: "GENERAL", label: "General" }, // no children → empty chip list
    ]);
    render(<Taxonomy />);
    expect(await screen.findByText("Disaster")).toBeInTheDocument();
    expect(screen.getByText("DISASTER.FLOOD")).toBeInTheDocument();
    expect(screen.getByText("General")).toBeInTheDocument();
  });
});
