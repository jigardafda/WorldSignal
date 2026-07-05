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
        // The lazy geo/boundary chunk is multi-MB; keep it out of precache and
        // cache it at runtime on first use instead (see runtimeCaching below).
        globIgnores: ["**/maps-*.js"],
        // SPA offline: serve the shell for navigations, but never for the API.
        navigateFallback: "/index.html",
        navigateFallbackDenylist: [/^\/v1/, /^\/graphql/],
        runtimeCaching: [
          {
            // OpenStreetMap basemap tiles — cache visited tiles for offline reuse.
            urlPattern: /^https:\/\/[a-c]\.tile\.openstreetmap\.org\/.*/i,
            handler: "CacheFirst",
            options: {
              cacheName: "osm-tiles",
              expiration: { maxEntries: 1500, maxAgeSeconds: 30 * 24 * 60 * 60 },
              cacheableResponse: { statuses: [0, 200] },
            },
          },
          {
            // The large boundary/geocoding chunk, cached on demand after first load.
            urlPattern: /\/assets\/maps-.*\.js$/,
            handler: "CacheFirst",
            options: {
              cacheName: "geo-data",
              expiration: { maxEntries: 3, maxAgeSeconds: 30 * 24 * 60 * 60 },
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
