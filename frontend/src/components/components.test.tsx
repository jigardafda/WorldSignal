import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../test/utils";
import { ConfidenceBar, SeverityBadge, StatusBadge } from "./badges";
import { AsyncBoundary, EmptyState, ErrorState, LoadingState } from "./States";
import { StatCard } from "./StatCard";
import { DataTable } from "./DataTable";
import { ExtLink, safeHref } from "./ExtLink";
import { PageHeader } from "./PageHeader";
import { ConfirmButton } from "./ConfirmButton";

describe("badges", () => {
  it("renders severity/status/confidence", () => {
    renderWithProviders(<div><SeverityBadge severity="HIGH" /><StatusBadge status="CONFIRMED" /><ConfidenceBar value={0.83} /></div>);
    expect(screen.getByText("HIGH")).toBeInTheDocument();
    expect(screen.getByText("CONFIRMED")).toBeInTheDocument();
    expect(screen.getByText("83%")).toBeInTheDocument();
  });
  it("falls back to gray for unknown values", () => {
    renderWithProviders(<><SeverityBadge severity="ZZZ" /><StatusBadge status="ZZZ" /></>);
    expect(screen.getAllByText("ZZZ")).toHaveLength(2);
  });
});

describe("States", () => {
  it("renders loading/error/empty", () => {
    const { rerender } = renderWithProviders(<LoadingState />);
    expect(screen.getByTestId("loading")).toBeInTheDocument();
    rerender(<EmptyState message="nothing" />);
    expect(screen.getByText("nothing")).toBeInTheDocument();
  });
  it("error retry button works", async () => {
    const onRetry = vi.fn();
    renderWithProviders(<ErrorState message="bad" onRetry={onRetry} />);
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(onRetry).toHaveBeenCalled();
  });
  it("AsyncBoundary picks the right state", () => {
    const base = { reload: () => {} };
    const { rerender } = renderWithProviders(<AsyncBoundary state={{ ...base, data: null, loading: true, error: null }}>{() => <div>x</div>}</AsyncBoundary>);
    expect(screen.getByTestId("loading")).toBeInTheDocument();
    rerender(<AsyncBoundary state={{ ...base, data: null, loading: false, error: "e" }}>{() => <div>x</div>}</AsyncBoundary>);
    expect(screen.getByTestId("error")).toBeInTheDocument();
    rerender(<AsyncBoundary state={{ ...base, data: null, loading: false, error: null }}>{(d) => <div>val:{String(d)}</div>}</AsyncBoundary>);
    expect(screen.getByText("val:null")).toBeInTheDocument(); // null passed through (detail "not found")
    rerender(<AsyncBoundary state={{ ...base, data: [], loading: false, error: null }} empty={(d: unknown[]) => d.length === 0}>{() => <div>x</div>}</AsyncBoundary>);
    expect(screen.getByTestId("empty")).toBeInTheDocument();
    rerender(<AsyncBoundary state={{ ...base, data: [1], loading: false, error: null }}>{() => <div>loaded</div>}</AsyncBoundary>);
    expect(screen.getByText("loaded")).toBeInTheDocument();
  });
});

describe("StatCard + PageHeader", () => {
  it("renders", () => {
    renderWithProviders(<StatCard label="Sources" value={5} />);
    expect(screen.getByText("Sources")).toBeInTheDocument();
    expect(screen.getByText("5")).toBeInTheDocument();
    renderWithProviders(<PageHeader title="Title" subtitle="Sub" actions={<button>act</button>} />);
    expect(screen.getByText("Title")).toBeInTheDocument();
    expect(screen.getByText("act")).toBeInTheDocument();
  });
});

describe("DataTable", () => {
  it("renders rows, handles clicks, and shows empty", async () => {
    const onClick = vi.fn();
    renderWithProviders(
      <DataTable rows={[{ id: "1", name: "Alpha" }]} getKey={(r) => r.id} onRowClick={onClick}
        columns={[{ key: "name", header: "Name", render: (r) => r.name }, { key: "id", header: "ID" }]} />,
    );
    expect(screen.getByText("Alpha")).toBeInTheDocument();
    await userEvent.click(screen.getByText("Alpha"));
    expect(onClick).toHaveBeenCalled();
    renderWithProviders(<DataTable rows={[]} getKey={(r: { id: string }) => r.id} columns={[]} emptyMessage="none" />);
    expect(screen.getByText("none")).toBeInTheDocument();
  });
});

describe("ExtLink / safeHref", () => {
  it("allows http(s) and blocks others", () => {
    expect(safeHref("https://x.com")).toBe("https://x.com");
    expect(safeHref("javascript:alert(1)")).toBeNull();
    expect(safeHref(null)).toBeNull();
    expect(safeHref("ftp://x.com")).toBeNull();
    expect(safeHref("http://[bad")).toBeNull(); // invalid URL → catch branch
    renderWithProviders(<ExtLink url="https://safe.example">safe</ExtLink>);
    expect(screen.getByRole("link", { name: "safe" })).toHaveAttribute("href", "https://safe.example");
    renderWithProviders(<ExtLink url="javascript:evil()">danger</ExtLink>);
    expect(screen.queryByRole("link", { name: "danger" })).toBeNull();
    expect(screen.getByText("danger")).toBeInTheDocument();
  });
});

describe("ConfirmButton", () => {
  it("confirms and runs the action", async () => {
    const onConfirm = vi.fn().mockResolvedValue(true);
    const onDone = vi.fn();
    renderWithProviders(<ConfirmButton label="Delete" message="sure?" onConfirm={onConfirm} onDone={onDone} />);
    await userEvent.click(screen.getByRole("button", { name: "Delete" }));
    expect(await screen.findByText("sure?")).toBeInTheDocument();
    await userEvent.click(await screen.findByRole("button", { name: "Confirm" }));
    await waitFor(() => expect(onConfirm).toHaveBeenCalled());
    expect(onDone).toHaveBeenCalled();
  });
  it("surfaces action errors", async () => {
    const onConfirm = vi.fn().mockRejectedValue(new Error("nope"));
    renderWithProviders(<ConfirmButton label="Del" message="sure?" onConfirm={onConfirm} />);
    await userEvent.click(screen.getByRole("button", { name: "Del" }));
    await userEvent.click(await screen.findByRole("button", { name: "Confirm" }));
    await waitFor(() => expect(onConfirm).toHaveBeenCalled());
  });
  it("cancels", async () => {
    const onConfirm = vi.fn();
    renderWithProviders(<ConfirmButton label="Del" message="sure?" onConfirm={onConfirm} />);
    await userEvent.click(screen.getByRole("button", { name: "Del" }));
    await userEvent.click(await screen.findByRole("button", { name: "Cancel" }));
    expect(onConfirm).not.toHaveBeenCalled();
  });
});
