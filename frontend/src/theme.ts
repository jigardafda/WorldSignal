import { createTheme, type MantineColorsTuple } from "@mantine/core";

// WorldSignal design system. The brand mark is a blue→cyan gradient
// (#2F6DF6 → #22C3E6); the theme carries that identity through the whole app:
// a saturated "brand" primary, a "signal" cyan accent, a geometric display face
// for headings, and softer, brand-tinted surfaces.

const brand: MantineColorsTuple = [
  "#eaf1ff", "#d3e0ff", "#a6bffe", "#7799fb", "#4f79f8",
  "#3568f6", "#2f6df6", "#2258db", "#1c4dc4", "#123f9e",
];

// "signal" — the cyan highlight used for accents, active states and gradients.
const signal: MantineColorsTuple = [
  "#dffaff", "#b6f1fc", "#89e8f8", "#5adef2", "#34d5ec",
  "#1fcbe4", "#22c3e6", "#00a6c7", "#0086a1", "#00647a",
];

export const theme = createTheme({
  primaryColor: "brand",
  primaryShade: { light: 6, dark: 5 },
  autoContrast: true,
  luminanceThreshold: 0.35,
  colors: { brand, signal },
  defaultRadius: "md",
  fontFamily: "'Inter Variable', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif",
  fontFamilyMonospace: "'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, monospace",
  headings: {
    fontFamily: "'Space Grotesk Variable', 'Inter Variable', sans-serif",
    fontWeight: "600",
    sizes: {
      h1: { fontWeight: "700" },
      h2: { fontWeight: "700" },
      h3: { fontWeight: "600" },
      h4: { fontWeight: "600" },
    },
  },
  // Softer, brand-tinted elevation so cards read as layered surfaces, not boxes.
  shadows: {
    xs: "0 1px 2px rgba(16, 33, 74, 0.06)",
    sm: "0 2px 6px rgba(16, 33, 74, 0.07), 0 1px 2px rgba(16, 33, 74, 0.05)",
    md: "0 6px 18px rgba(16, 33, 74, 0.08), 0 2px 6px rgba(16, 33, 74, 0.05)",
    lg: "0 14px 40px rgba(16, 33, 74, 0.12), 0 4px 12px rgba(16, 33, 74, 0.06)",
    xl: "0 24px 60px rgba(16, 33, 74, 0.16)",
  },
  components: {
    Paper: { defaultProps: { radius: "lg" } },
    Card: { defaultProps: { radius: "lg" } },
    Button: { defaultProps: { radius: "md" } },
    Badge: { defaultProps: { radius: "sm" } },
    Modal: { defaultProps: { radius: "lg" } },
  },
});
