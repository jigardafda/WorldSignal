import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../test/utils";

const { apiMock } = vi.hoisted(() => ({ apiMock: { countries: vi.fn() } }));
vi.mock("../lib/api", () => ({ api: apiMock }));

import { CountrySelect } from "./CountrySelect";
import { _resetCountriesCache, countryDisplay, countryLabel } from "../lib/countries";

const COUNTRIES = [
  { code: "US", name: "United States", flag: "🇺🇸", currency: "USD", capital: "Washington, D.C.", capitalLat: 38.9, capitalLng: -77 },
  { code: "GB", name: "United Kingdom", flag: "🇬🇧", currency: "GBP", capital: "London", capitalLat: 51.5, capitalLng: -0.12 },
];

beforeEach(() => { _resetCountriesCache(); apiMock.countries.mockResolvedValue(COUNTRIES); });
afterEach(() => vi.clearAllMocks());

describe("countries lib", () => {
  it("formats labels and displays", () => {
    expect(countryLabel(COUNTRIES[0])).toBe("🇺🇸 United States");
    const byCode = { US: COUNTRIES[0] };
    expect(countryDisplay("US", byCode)).toBe("🇺🇸 United States");
    expect(countryDisplay("ZZ", byCode)).toBe("ZZ"); // unknown → raw code
    expect(countryDisplay(null, byCode)).toBe("—");
  });
});

describe("CountrySelect", () => {
  it("loads options from the backend and selects by code", async () => {
    const onChange = vi.fn();
    renderWithProviders(<CountrySelect value={null} onChange={onChange} label="Country" data-testid="cs" />);
    // Options come from api.countries (full name + flag in brackets).
    await waitFor(() => expect(apiMock.countries).toHaveBeenCalled());
    await userEvent.click(screen.getByTestId("cs"));
    await userEvent.click(await screen.findByText("🇬🇧 United Kingdom"));
    await waitFor(() => expect(onChange).toHaveBeenCalledWith("GB", expect.anything()));
  });

  it("shows the empty fallback when the list fails to load", async () => {
    apiMock.countries.mockRejectedValue(new Error("down"));
    renderWithProviders(<CountrySelect value={null} onChange={vi.fn()} label="Country" data-testid="cs" />);
    // Renders without crashing even though the list failed to load.
    expect(await screen.findByTestId("cs")).toBeInTheDocument();
  });
});
