import { fireEvent, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../test/utils";
import { FilterBuilder } from "./FilterBuilder";
import type { SubFilter } from "../lib/subFilter";

vi.mock("../lib/countries", () => ({
  useCountries: () => ({ list: [{ code: "US", name: "United States", flag: "🇺🇸" }], byCode: {}, loading: false }),
}));

function Harness({ initial = {} as SubFilter }) {
  const [f, setF] = useState<SubFilter>(initial);
  return (
    <>
      <FilterBuilder value={f} onChange={setF} />
      <div data-testid="state">{JSON.stringify(f)}</div>
    </>
  );
}
const state = () => screen.getByTestId("state").textContent ?? "";

describe("FilterBuilder", () => {
  it("edits the keyword", async () => {
    renderWithProviders(<Harness />);
    await userEvent.type(screen.getByTestId("filter-keyword"), "quake");
    expect(state()).toContain('"keyword":"quake"');
  });

  it("selects a category and a minimum severity", async () => {
    renderWithProviders(<Harness />);
    await userEvent.click(screen.getByPlaceholderText("Any category"));
    await userEvent.click(await screen.findByRole("option", { name: "Politics", hidden: true }));
    expect(state()).toContain("POLITICS");

    await userEvent.click(screen.getByTestId("filter-minSeverity"));
    await userEvent.click(await screen.findByRole("option", { name: "CRITICAL", hidden: true }));
    expect(state()).toContain('"minSeverity":"CRITICAL"');
  });

  it("toggles to JSON view and edits raw JSON", () => {
    renderWithProviders(<Harness initial={{ minSeverity: "HIGH" }} />);
    fireEvent.click(screen.getByTestId("filter-json-toggle"));
    const json = screen.getByTestId("filter-json") as HTMLTextAreaElement;
    expect(json.value).toContain('"minSeverity": "HIGH"');
    fireEvent.change(json, { target: { value: '{"countries":["US"]}' } });
    expect(state()).toContain('"countries":["US"]');
    // invalid JSON is ignored (keeps last valid value)
    fireEvent.change(json, { target: { value: "{bad" } });
    expect(state()).toContain('"countries":["US"]');
  });
});
