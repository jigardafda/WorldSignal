import { MantineProvider } from "@mantine/core";
import { Notifications } from "@mantine/notifications";
import { render, type RenderResult } from "@testing-library/react";
import type { ReactElement, ReactNode } from "react";
import { MemoryRouter, Route, Routes } from "react-router-dom";

/** Render a component wrapped in Mantine + Notifications + a memory router. The
 * providers are supplied via RTL's `wrapper` so `rerender` keeps them. */
export function renderWithProviders(
  ui: ReactElement,
  { route = "/", path }: { route?: string; path?: string } = {},
): RenderResult {
  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <MantineProvider>
        <Notifications />
        <MemoryRouter initialEntries={[route]}>{children}</MemoryRouter>
      </MantineProvider>
    );
  }
  const element = path ? <Routes><Route path={path} element={ui} /></Routes> : ui;
  return render(element, { wrapper: Wrapper });
}
