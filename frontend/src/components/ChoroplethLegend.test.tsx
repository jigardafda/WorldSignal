import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../test/utils";
import { ChoroplethLegend } from "./ChoroplethLegend";

describe("ChoroplethLegend", () => {
  it("shows a sequential scale with the count domain for the count metric", () => {
    renderWithProviders(<ChoroplethLegend metric="count" max={42} />);
    const el = screen.getByTestId("choropleth-legend");
    expect(el).toHaveTextContent("Signals per country");
    expect(el).toHaveTextContent("1");
    expect(el).toHaveTextContent("42");
  });

  it("labels the sentiment scale as diverging", () => {
    renderWithProviders(<ChoroplethLegend metric="sentiment" max={1} />);
    const el = screen.getByTestId("choropleth-legend");
    expect(el).toHaveTextContent("Net sentiment");
    expect(el).toHaveTextContent("Negative");
    expect(el).toHaveTextContent("Positive");
  });
});
