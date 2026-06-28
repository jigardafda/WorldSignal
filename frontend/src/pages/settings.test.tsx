import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../test/utils";

const { apiMock } = vi.hoisted(() => ({
  apiMock: {
    llmStatus: vi.fn(), llmKeys: vi.fn(), llmModels: vi.fn(), createLLMKey: vi.fn(),
    setActiveLLMKey: vi.fn(), testLLMKey: vi.fn(), deleteLLMKey: vi.fn(),
  },
}));
vi.mock("../lib/api", () => ({ api: apiMock }));

import { Settings } from "./Settings";

beforeEach(() => {
  // Model options come from the provider; default to a non-empty live list.
  apiMock.llmModels.mockResolvedValue(["gpt-4o", "gpt-4o-mini", "o3-mini"]);
});

const status = (o = {}) => ({ provider: "OPENAI", enabled: true, source: "ENV", model: "gpt-4o-mini", hasSystemKey: true, activeLabel: null, ...o });
const key = (o = {}) => ({ id: "k1", provider: "OPENAI", label: "Prod", keyLast4: "7890", model: "gpt-4o", isActive: false, status: "VALID", lastTestedAt: "2026-01-01T00:00:00Z", lastError: null, createdBy: "admin", createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-01-01T00:00:00Z", ...o });

afterEach(() => vi.clearAllMocks());

describe("Settings (LLM keys)", () => {
  it("shows the effective LLM status and the masked key list", async () => {
    apiMock.llmStatus.mockResolvedValue(status({ source: "DB", activeLabel: "Prod" }));
    apiMock.llmKeys.mockResolvedValue([key({ isActive: true })]);
    renderWithProviders(<Settings />);
    expect(await screen.findByTestId("llm-status")).toBeInTheDocument();
    expect(screen.getByText("LLM enrichment is active")).toBeInTheDocument();
    expect(screen.getByText("••••7890")).toBeInTheDocument();
    expect(screen.getByText("active")).toBeInTheDocument();
  });

  it("shows a disabled banner + empty state when no key is configured", async () => {
    apiMock.llmStatus.mockResolvedValue(status({ enabled: false, source: "NONE", hasSystemKey: false }));
    apiMock.llmKeys.mockResolvedValue([]);
    renderWithProviders(<Settings />);
    expect(await screen.findByText("LLM enrichment is disabled")).toBeInTheDocument();
    expect(await screen.findByText(/No admin keys yet/)).toBeInTheDocument();
  });

  it("adds and validates a key", async () => {
    apiMock.llmStatus.mockResolvedValue(status());
    apiMock.llmKeys.mockResolvedValue([]);
    apiMock.createLLMKey.mockResolvedValue(key({ status: "VALID" }));
    renderWithProviders(<Settings />);
    await screen.findByText("Admin-managed keys");
    await userEvent.click(screen.getByRole("button", { name: "Add OpenAI key" }));
    await userEvent.type(await screen.findByTestId("llm-label"), "Prod");
    await userEvent.type(screen.getByTestId("llm-key"), "sk-test-1234567890");
    await userEvent.click(screen.getByRole("button", { name: /Add & validate/ }));
    await waitFor(() => expect(apiMock.createLLMKey).toHaveBeenCalledWith(expect.objectContaining({ label: "Prod", key: "sk-test-1234567890" })));
  });

  it("lets you pick a model from the dropdown", async () => {
    apiMock.llmStatus.mockResolvedValue(status());
    apiMock.llmKeys.mockResolvedValue([]);
    apiMock.createLLMKey.mockResolvedValue(key({ model: "gpt-4o" }));
    renderWithProviders(<Settings />);
    await screen.findByText("Admin-managed keys");
    await userEvent.click(screen.getByRole("button", { name: "Add OpenAI key" }));
    await userEvent.type(await screen.findByTestId("llm-label"), "Prod");
    await userEvent.type(screen.getByTestId("llm-key"), "sk-test-1234567890");
    await userEvent.click(screen.getByTestId("llm-model"));
    await userEvent.click(await screen.findByRole("option", { name: "gpt-4o" }));
    await userEvent.click(screen.getByRole("button", { name: /Add & validate/ }));
    await waitFor(() => expect(apiMock.createLLMKey).toHaveBeenCalledWith(expect.objectContaining({ model: "gpt-4o" })));
  });

  it("blocks adding a key with a too-short value", async () => {
    apiMock.llmStatus.mockResolvedValue(status());
    apiMock.llmKeys.mockResolvedValue([]);
    renderWithProviders(<Settings />);
    await userEvent.click(screen.getByRole("button", { name: "Add OpenAI key" }));
    await userEvent.type(await screen.findByTestId("llm-label"), "X");
    await userEvent.type(screen.getByTestId("llm-key"), "short");
    await userEvent.click(screen.getByRole("button", { name: /Add & validate/ }));
    expect(await screen.findByText("Enter a valid API key")).toBeInTheDocument();
    expect(apiMock.createLLMKey).not.toHaveBeenCalled();
  });

  it("activates, tests and deletes a key", async () => {
    apiMock.llmStatus.mockResolvedValue(status());
    apiMock.llmKeys.mockResolvedValue([key()]);
    apiMock.setActiveLLMKey.mockResolvedValue(key({ isActive: true }));
    apiMock.testLLMKey.mockResolvedValue({ ok: true, status: "VALID" });
    apiMock.deleteLLMKey.mockResolvedValue(true);
    renderWithProviders(<Settings />);
    await screen.findByText("Prod");
    await userEvent.click(screen.getByRole("button", { name: "Set active" }));
    await waitFor(() => expect(apiMock.setActiveLLMKey).toHaveBeenCalledWith("k1"));
    await userEvent.click(screen.getByRole("button", { name: "Test" }));
    await waitFor(() => expect(apiMock.testLLMKey).toHaveBeenCalledWith("k1"));
    await userEvent.click(screen.getByRole("button", { name: "Delete" }));
    const dialog = await screen.findByRole("dialog");
    await userEvent.click(within(dialog).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(apiMock.deleteLLMKey).toHaveBeenCalledWith("k1"));
  });

  it("surfaces a failed key test", async () => {
    apiMock.llmStatus.mockResolvedValue(status());
    apiMock.llmKeys.mockResolvedValue([key({ status: "INVALID" })]);
    apiMock.testLLMKey.mockResolvedValue({ ok: false, status: "INVALID", error: "provider rejected key (HTTP 401)" });
    renderWithProviders(<Settings />);
    await screen.findByText("Prod");
    await userEvent.click(screen.getByRole("button", { name: "Test" }));
    await waitFor(() => expect(apiMock.testLLMKey).toHaveBeenCalled());
  });

  it("surfaces errors from create, activate and test", async () => {
    apiMock.llmStatus.mockResolvedValue(status());
    apiMock.llmKeys.mockResolvedValue([key()]);
    apiMock.createLLMKey.mockRejectedValue(new Error("dup"));
    apiMock.setActiveLLMKey.mockRejectedValue(new Error("nope"));
    apiMock.testLLMKey.mockRejectedValue(new Error("network"));
    renderWithProviders(<Settings />);
    await screen.findByText("Prod");
    // act() error path.
    await userEvent.click(screen.getByRole("button", { name: "Set active" }));
    await waitFor(() => expect(apiMock.setActiveLLMKey).toHaveBeenCalled());
    // test() error path.
    await userEvent.click(screen.getByRole("button", { name: "Test" }));
    await waitFor(() => expect(apiMock.testLLMKey).toHaveBeenCalled());
    // create() error path.
    await userEvent.click(screen.getByRole("button", { name: "Add OpenAI key" }));
    await userEvent.type(await screen.findByTestId("llm-label"), "Prod");
    await userEvent.type(screen.getByTestId("llm-key"), "sk-test-1234567890");
    await userEvent.click(screen.getByRole("button", { name: /Add & validate/ }));
    await waitFor(() => expect(apiMock.createLLMKey).toHaveBeenCalled());
  });
});
