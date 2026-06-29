/// <reference types="vite/client" />

// Allow side-effect imports of stylesheets (e.g. Mantine's bundled CSS).
// TypeScript 6 resolves package `exports` strictly and otherwise can't find
// these `*.css` subpaths.
declare module "*.css";

// world-atlas ships a large TopoJSON; declare it lightly so TS doesn't parse it.
declare module "world-atlas/countries-110m.json" {
  const topology: unknown;
  export default topology;
}
