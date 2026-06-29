import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Backend target is overridable (e2e runs the Go backend on a separate port).
const backend = process.env.WS_BACKEND ?? "http://localhost:4000";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/v1": backend,
      "/graphql": backend,
    },
  },
  build: {
    // Vendor chunks below are intentionally grouped for long-term caching; the
    // React+router core sits just above 500kB (≈150kB gzipped), so raise the
    // advisory limit rather than over-split into circular chunks.
    chunkSizeWarningLimit: 700,
    rollupOptions: {
      output: {
        // Split large vendors into separate cacheable chunks so no single chunk
        // dominates the initial payload (and to clear the chunk-size warning).
        manualChunks(id) {
          if (!id.includes("node_modules")) return undefined;
          // Boundary geometry/converter/ISO map are only dynamically imported by
          // the live map's country outline — keep them in a lazily-loaded chunk.
          if (id.includes("world-atlas") || id.includes("topojson") || id.includes("i18n-iso-countries") || id.includes("country-state-city")) return "maps";
          if (id.includes("recharts") || id.includes("d3-") || id.includes("@mantine/charts")) return "charts";
          if (id.includes("@tabler")) return "icons";
          if (id.includes("@mantine")) return "mantine";
          // Everything else (react, react-dom, react-router, etc.) stays together
          // to avoid circular cross-chunk references.
          return "vendor";
        },
      },
    },
  },
});
