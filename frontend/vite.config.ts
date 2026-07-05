import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { VitePWA } from "vite-plugin-pwa";

// Backend target is overridable (e2e runs the Go backend on a separate port).
const backend = process.env.WS_BACKEND ?? "http://localhost:4000";

export default defineConfig({
  plugins: [
    react(),
    // Service worker: precache the app shell (instant/offline load) and
    // runtime-cache OpenStreetMap tiles as they're viewed so already-seen map
    // areas render offline. Reuses the existing static manifest.webmanifest.
    VitePWA({
      registerType: "autoUpdate",
      injectRegister: "auto",
      manifest: false,
      devOptions: { enabled: true, type: "module" },
      workbox: {
        globPatterns: ["**/*.{js,css,html,svg,png,ico,woff2}"],
        // The lazy geo/boundary and 3D-globe (three.js) chunks are multi-MB; keep
        // them out of precache and cache at runtime on first use (runtimeCaching).
        globIgnores: ["**/maps-*.js", "**/globe-*.js"],
        // SPA offline: serve the shell for navigations, but never for the API.
        navigateFallback: "/index.html",
        navigateFallbackDenylist: [/^\/v1/, /^\/graphql/],
        runtimeCaching: [
          {
            // CARTO basemap tiles — cache visited tiles for offline reuse.
            urlPattern: /^https:\/\/[a-d]\.basemaps\.cartocdn\.com\/.*/i,
            handler: "CacheFirst",
            options: {
              cacheName: "basemap-tiles",
              expiration: { maxEntries: 1500, maxAgeSeconds: 30 * 24 * 60 * 60 },
              cacheableResponse: { statuses: [0, 200] },
            },
          },
          {
            // The large boundary/geocoding and 3D-globe chunks, cached on demand.
            urlPattern: /\/assets\/(maps|globe)-.*\.js$/,
            handler: "CacheFirst",
            options: {
              cacheName: "geo-data",
              expiration: { maxEntries: 5, maxAgeSeconds: 30 * 24 * 60 * 60 },
              cacheableResponse: { statuses: [0, 200] },
            },
          },
        ],
      },
    }),
  ],
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
          // three.js / globe.gl are only used by the lazily-loaded 3D globe view —
          // keep them out of the eager vendor chunk so 2D-map users never pay for them.
          if (id.includes("/three") || id.includes("globe.gl") || id.includes("react-globe.gl") || id.includes("kapsule") || id.includes("accessor-fn") || id.includes("three-")) return "globe";
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
