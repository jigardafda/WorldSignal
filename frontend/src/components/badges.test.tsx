import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ConfidenceBar, SeverityBadge, StatusBadge } from "./badges";

describe("badges", () => {
  it("SeverityBadge renders label and lowercased class", () => {
    const { container } = render(<SeverityBadge severity="HIGH" />);
    expect(screen.getByText("HIGH")).toBeInTheDocument();
    expect(container.querySelector(".sev-high")).toBeTruthy();
  });

  it("StatusBadge renders label and lowercased class", () => {
    const { container } = render(<StatusBadge status="CONFIRMED" />);
    expect(screen.getByText("CONFIRMED")).toBeInTheDocument();
    expect(container.querySelector(".status-confirmed")).toBeTruthy();
  });

  it("ConfidenceBar renders rounded percentage and width", () => {
    const { container } = render(<ConfidenceBar value={0.826} />);
    expect(screen.getByText("83%")).toBeInTheDocument();
    const fill = container.querySelector(".conf-fill") as HTMLElement;
    expect(fill.style.width).toBe("83%");
  });
});
