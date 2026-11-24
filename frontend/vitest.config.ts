import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "path";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "happy-dom",
    // Node 25+ has a global localStorage that is undefined without
    // --localstorage-file, and it shadows happy-dom's.
    execArgv: ["--no-experimental-webstorage"],
    setupFiles: ["./vitest.setup.ts"],
    exclude: ["**/node_modules/**", "**/dist/**", "e2e/**"],
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
});
