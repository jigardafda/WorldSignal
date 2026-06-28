import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../test/utils";
import { Login } from "./Login";

const { authMock } = vi.hoisted(() => ({
  authMock: { user: null as unknown, loading: false, login: vi.fn(), logout: vi.fn(), refresh: vi.fn(), hasPerm: () => true },
}));
vi.mock("../lib/auth", () => ({ useAuth: () => authMock }));

afterEach(() => {
  authMock.user = null;
  authMock.login.mockReset();
});

describe("Login", () => {
  it("validates inputs", async () => {
    renderWithProviders(<Login />, { route: "/login" });
    await userEvent.click(screen.getByRole("button", { name: "Sign in" }));
    expect(await screen.findByText("Enter a valid email")).toBeInTheDocument();
    expect(authMock.login).not.toHaveBeenCalled();
  });

  it("submits valid credentials", async () => {
    authMock.login.mockResolvedValue(undefined);
    renderWithProviders(<Login />, { route: "/login" });
    await userEvent.type(screen.getByTestId("email"), "a@b.c");
    await userEvent.type(screen.getByTestId("password"), "secret");
    await userEvent.click(screen.getByRole("button", { name: "Sign in" }));
    await waitFor(() => expect(authMock.login).toHaveBeenCalledWith("a@b.c", "secret"));
  });

  it("shows an error when login fails", async () => {
    authMock.login.mockRejectedValue(new Error("invalid credentials"));
    renderWithProviders(<Login />, { route: "/login" });
    await userEvent.type(screen.getByTestId("email"), "a@b.c");
    await userEvent.type(screen.getByTestId("password"), "x");
    await userEvent.click(screen.getByRole("button", { name: "Sign in" }));
    expect(await screen.findByTestId("login-error")).toHaveTextContent("invalid credentials");
  });

  it("redirects when already authenticated", () => {
    authMock.user = { email: "a@b.c" };
    const { container } = renderWithProviders(<Login />, { route: "/login" });
    // Navigate renders nothing.
    expect(container.querySelector("[data-testid='email']")).toBeNull();
  });
});
