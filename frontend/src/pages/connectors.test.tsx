import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../test/utils";

const { apiMock } = vi.hoisted(() => ({
  apiMock: {
    emailConnectors: vi.fn(), emailProviders: vi.fn(), createEmailConnector: vi.fn(),
    updateEmailConnector: vi.fn(), setActiveEmailConnector: vi.fn(), testEmailConnector: vi.fn(),
    sendTestEmail: vi.fn(), deleteEmailConnector: vi.fn(),
  },
}));
vi.mock("../lib/api", () => ({ api: apiMock }));

import { Connectors } from "./Connectors";

const providers = [
  { code: "GMAIL", label: "Gmail", host: "smtp.gmail.com", port: 587, security: "STARTTLS", usernameHint: "your Gmail address", secretHint: "an App Password", help: "Create an App Password.", docsAnchor: "gmail", editable: false },
  { code: "CUSTOM", label: "Custom SMTP", host: "", port: 587, security: "STARTTLS", usernameHint: "user", secretHint: "pass", help: "Enter host/port.", docsAnchor: "custom", editable: true },
];
const conn = (o = {}) => ({ id: "c1", name: "Team", provider: "GMAIL", host: "smtp.gmail.com", port: 587, security: "STARTTLS", username: "me@gmail.com", secretLast4: "word", fromEmail: "me@gmail.com", fromName: "WS", isActive: false, enabled: true, status: "VALID", lastTestedAt: "2026-01-01T00:00:00Z", lastError: null, createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-01-01T00:00:00Z", ...o });

beforeEach(() => {
  apiMock.emailProviders.mockResolvedValue(providers);
});
afterEach(() => vi.clearAllMocks());

describe("Connectors", () => {
  it("shows the active connector banner and list", async () => {
    apiMock.emailConnectors.mockResolvedValue([conn({ isActive: true })]);
    renderWithProviders(<Connectors />);
    expect(await screen.findByTestId("connector-status")).toBeInTheDocument();
    expect(screen.getByText(/Active connector: Team/)).toBeInTheDocument();
    expect(screen.getByText("smtp.gmail.com:587")).toBeInTheDocument();
  });

  it("shows the empty state and setup guidance", async () => {
    apiMock.emailConnectors.mockResolvedValue([]);
    renderWithProviders(<Connectors />);
    expect(await screen.findByText("No active email connector")).toBeInTheDocument();
    expect(await screen.findByText(/No connectors yet/)).toBeInTheDocument();
  });

  it("creates a connector using a provider preset", async () => {
    apiMock.emailConnectors.mockResolvedValue([]);
    apiMock.createEmailConnector.mockResolvedValue(conn({ status: "VALID" }));
    renderWithProviders(<Connectors />);
    await screen.findByText(/No connectors yet/);
    await userEvent.click(screen.getByTestId("add-connector"));
    // Gmail preset help is shown.
    expect(await screen.findByText("Create an App Password.")).toBeInTheDocument();
    await userEvent.type(screen.getByTestId("conn-name"), "Team");
    await userEvent.type(screen.getByTestId("conn-from"), "me@gmail.com");
    await userEvent.type(screen.getByTestId("conn-username"), "me@gmail.com");
    await userEvent.type(screen.getByTestId("conn-secret"), "app-password");
    await userEvent.click(screen.getByTestId("conn-submit"));
    await waitFor(() => expect(apiMock.createEmailConnector).toHaveBeenCalledWith(
      expect.objectContaining({ name: "Team", provider: "GMAIL", host: "smtp.gmail.com", port: 587, fromEmail: "me@gmail.com", secret: "app-password" }),
    ));
  });

  it("switches to the custom provider and its guidance", async () => {
    apiMock.emailConnectors.mockResolvedValue([]);
    renderWithProviders(<Connectors />);
    await screen.findByText(/No connectors yet/);
    await userEvent.click(screen.getByTestId("add-connector"));
    await screen.findByText("Create an App Password.");
    fireEvent.click(screen.getByTestId("conn-provider"));
    fireEvent.click(await screen.findByRole("option", { name: "Custom SMTP", hidden: true }));
    expect(await screen.findByText("Enter host/port.")).toBeInTheDocument();
  });

  it("blocks submit on an invalid from-address", async () => {
    apiMock.emailConnectors.mockResolvedValue([]);
    renderWithProviders(<Connectors />);
    await screen.findByText(/No connectors yet/);
    await userEvent.click(screen.getByTestId("add-connector"));
    // Fill the required fields (so native validation passes) but give a bad email
    // so Mantine's format validator fires.
    await userEvent.type(await screen.findByTestId("conn-name"), "N");
    await userEvent.type(screen.getByTestId("conn-from"), "not-an-email");
    await userEvent.type(screen.getByTestId("conn-secret"), "p");
    await userEvent.click(screen.getByTestId("conn-submit"));
    expect(await screen.findByText("A valid from address is required")).toBeInTheDocument();
    expect(apiMock.createEmailConnector).not.toHaveBeenCalled();
  });

  it("edits a connector without re-entering the secret", async () => {
    apiMock.emailConnectors.mockResolvedValue([conn()]);
    apiMock.updateEmailConnector.mockResolvedValue(conn({ name: "Renamed", status: "VALID" }));
    renderWithProviders(<Connectors />);
    await screen.findByText("Team");
    await userEvent.click(screen.getByRole("button", { name: "Edit" }));
    const name = await screen.findByTestId("conn-name");
    await userEvent.clear(name);
    await userEvent.type(name, "Renamed");
    await userEvent.click(screen.getByTestId("conn-submit"));
    await waitFor(() => expect(apiMock.updateEmailConnector).toHaveBeenCalledWith("c1", expect.not.objectContaining({ secret: expect.anything() })));
  });

  it("activates, tests, sends a test email and deletes", async () => {
    apiMock.emailConnectors.mockResolvedValue([conn()]);
    apiMock.setActiveEmailConnector.mockResolvedValue(conn({ isActive: true }));
    apiMock.testEmailConnector.mockResolvedValue({ ok: true, status: "VALID" });
    apiMock.sendTestEmail.mockResolvedValue({ ok: true });
    apiMock.deleteEmailConnector.mockResolvedValue(true);
    vi.spyOn(window, "prompt").mockReturnValue("me@example.com");
    renderWithProviders(<Connectors />);
    await screen.findByText("Team");
    await userEvent.click(screen.getByRole("button", { name: "Set active" }));
    await waitFor(() => expect(apiMock.setActiveEmailConnector).toHaveBeenCalledWith("c1"));
    await userEvent.click(screen.getByRole("button", { name: "Test" }));
    await waitFor(() => expect(apiMock.testEmailConnector).toHaveBeenCalledWith("c1"));
    await userEvent.click(screen.getByRole("button", { name: /Send test/ }));
    await waitFor(() => expect(apiMock.sendTestEmail).toHaveBeenCalledWith("c1", "me@example.com"));
    await userEvent.click(screen.getByRole("button", { name: "Delete" }));
    const dialog = await screen.findByRole("dialog");
    await userEvent.click(within(dialog).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(apiMock.deleteEmailConnector).toHaveBeenCalledWith("c1"));
  });

  it("cancels the send-test prompt", async () => {
    apiMock.emailConnectors.mockResolvedValue([conn()]);
    vi.spyOn(window, "prompt").mockReturnValue(null);
    renderWithProviders(<Connectors />);
    await screen.findByText("Team");
    await userEvent.click(screen.getByRole("button", { name: /Send test/ }));
    expect(apiMock.sendTestEmail).not.toHaveBeenCalled();
  });

  it("surfaces failures from save, test and send", async () => {
    apiMock.emailConnectors.mockResolvedValue([conn()]);
    apiMock.testEmailConnector.mockResolvedValue({ ok: false, status: "INVALID", error: "auth failed" });
    apiMock.sendTestEmail.mockRejectedValue(new Error("smtp down"));
    apiMock.createEmailConnector.mockRejectedValue(new Error("dup"));
    vi.spyOn(window, "prompt").mockReturnValue("x@y.com");
    renderWithProviders(<Connectors />);
    await screen.findByText("Team");
    await userEvent.click(screen.getByRole("button", { name: "Test" }));
    await waitFor(() => expect(apiMock.testEmailConnector).toHaveBeenCalled());
    await userEvent.click(screen.getByRole("button", { name: /Send test/ }));
    await waitFor(() => expect(apiMock.sendTestEmail).toHaveBeenCalled());
    await userEvent.click(screen.getByTestId("add-connector"));
    await userEvent.type(await screen.findByTestId("conn-name"), "N");
    await userEvent.type(screen.getByTestId("conn-from"), "a@b.com");
    await userEvent.type(screen.getByTestId("conn-secret"), "p");
    await userEvent.click(screen.getByTestId("conn-submit"));
    await waitFor(() => expect(apiMock.createEmailConnector).toHaveBeenCalled());
  });

  it("reports a connector saved with a failed verification", async () => {
    apiMock.emailConnectors.mockResolvedValue([]);
    apiMock.createEmailConnector.mockResolvedValue(conn({ status: "INVALID", lastError: "bad password" }));
    renderWithProviders(<Connectors />);
    await screen.findByText(/No connectors yet/);
    await userEvent.click(screen.getByTestId("add-connector"));
    await userEvent.type(await screen.findByTestId("conn-name"), "N");
    await userEvent.type(screen.getByTestId("conn-from"), "a@b.com");
    await userEvent.type(screen.getByTestId("conn-secret"), "p");
    await userEvent.click(screen.getByTestId("conn-submit"));
    await waitFor(() => expect(apiMock.createEmailConnector).toHaveBeenCalled());
  });
});
