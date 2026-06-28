import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { useAsync } from "./useAsync";

function Probe({ fn }: { fn: () => Promise<string> }) {
  const { data, loading, error, reload } = useAsync(fn, []);
  return (
    <div>
      <span data-testid="state">{loading ? "loading" : error ? `error:${error}` : `data:${data}`}</span>
      <button onClick={reload}>reload</button>
    </div>
  );
}

describe("useAsync", () => {
  it("resolves data", async () => {
    render(<Probe fn={() => Promise.resolve("hi")} />);
    expect(screen.getByTestId("state")).toHaveTextContent("loading");
    await waitFor(() => expect(screen.getByTestId("state")).toHaveTextContent("data:hi"));
  });

  it("captures errors", async () => {
    render(<Probe fn={() => Promise.reject(new Error("boom"))} />);
    await waitFor(() => expect(screen.getByTestId("state")).toHaveTextContent("error:boom"));
  });

  it("reloads on demand", async () => {
    const fn = vi.fn().mockResolvedValueOnce("a").mockResolvedValueOnce("b");
    render(<Probe fn={fn} />);
    await waitFor(() => expect(screen.getByTestId("state")).toHaveTextContent("data:a"));
    await userEvent.click(screen.getByText("reload"));
    await waitFor(() => expect(screen.getByTestId("state")).toHaveTextContent("data:b"));
  });

  it("stringifies non-Error rejections", async () => {
    render(<Probe fn={() => Promise.reject("plain")} />);
    await waitFor(() => expect(screen.getByTestId("state")).toHaveTextContent("error:plain"));
  });
});
