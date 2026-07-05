import { screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../test/utils";
import { ErrorBoundary } from "./ErrorBoundary";

const Boom = () => {
  throw new Error("boom");
};

afterEach(() => vi.restoreAllMocks());

describe("ErrorBoundary", () => {
  it("renders its children when nothing throws", () => {
    renderWithProviders(<ErrorBoundary><div>all good</div></ErrorBoundary>);
    expect(screen.getByText("all good")).toBeInTheDocument();
    expect(screen.queryByTestId("error-boundary")).toBeNull();
  });

  it("shows a fallback (not a blank screen) when a child throws", () => {
    vi.spyOn(console, "error").mockImplementation(() => {}); // silence React's error log
    renderWithProviders(<ErrorBoundary><Boom /></ErrorBoundary>);
    expect(screen.getByTestId("error-boundary")).toBeInTheDocument();
    expect(screen.getByText("Something went wrong")).toBeInTheDocument();
    expect(screen.getByTestId("error-reload")).toBeInTheDocument();
  });

  it("recovers automatically when resetKey changes (e.g. route navigation)", () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    const { rerender } = renderWithProviders(<ErrorBoundary resetKey="/live"><Boom /></ErrorBoundary>);
    expect(screen.getByTestId("error-boundary")).toBeInTheDocument();
    rerender(<ErrorBoundary resetKey="/dashboard"><div>recovered</div></ErrorBoundary>);
    expect(screen.getByText("recovered")).toBeInTheDocument();
    expect(screen.queryByTestId("error-boundary")).toBeNull();
  });
});
