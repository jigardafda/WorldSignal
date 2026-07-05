import { fireEvent, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../test/utils";
import { CodeExamples } from "./CodeExamples";

const opts = { baseUrl: "https://api.example.com", subscriptionId: "sub_9" };

describe("CodeExamples", () => {
  it("shows a consumer snippet embedding the channel endpoint", () => {
    renderWithProviders(<CodeExamples channel="SSE" opts={opts} />);
    expect(screen.getByTestId("code-block").textContent).toContain("/v1/stream/sse?subscription=sub_9");
  });

  it("switches the snippet when a language tab is clicked", () => {
    renderWithProviders(<CodeExamples channel="POLLING" opts={opts} />);
    expect(screen.getByTestId("code-block").textContent).toContain("urllib.request"); // default: python
    fireEvent.click(screen.getByTestId("code-tab-go"));
    expect(screen.getByTestId("code-block").textContent).toContain("http.NewRequest");
    fireEvent.click(screen.getByTestId("code-tab-ruby"));
    expect(screen.getByTestId("code-block").textContent).toContain("Net::HTTP");
  });

  it("shows the webhook hint for the WEBHOOK channel", () => {
    renderWithProviders(<CodeExamples channel="WEBHOOK" opts={opts} />);
    expect(screen.getByTestId("code-examples").textContent).toContain("verifies the signature");
  });

  it("renders a note (no code block) for EMAIL", () => {
    renderWithProviders(<CodeExamples channel="EMAIL" opts={opts} />);
    expect(screen.getByTestId("code-examples").textContent).toContain("no client");
    expect(screen.queryByTestId("code-block")).toBeNull();
  });
});
