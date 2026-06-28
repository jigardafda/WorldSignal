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
});
