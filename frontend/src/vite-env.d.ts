/// <reference types="vite/client" />

// Allow side-effect imports of stylesheets (e.g. Mantine's bundled CSS).
// TypeScript 6 resolves package `exports` strictly and otherwise can't find
// these `*.css` subpaths.
declare module "*.css";
