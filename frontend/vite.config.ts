/// <reference types="vitest/config" />
import path from "node:path";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import wails from "@wailsio/runtime/plugins/vite";

// Set by `wails3 task dev:browser`: the Go backend runs in Wails server mode and Vite forwards the runtime
// calls, events and streams (everything under /wails) to it, so the UI works in a normal browser.
const serverModeBackend = process.env.DOCKLET_SERVER_URL;

export default defineConfig({
  resolve: { alias: { "@": path.resolve(__dirname, "./src"), "@bindings": path.resolve(__dirname, "./bindings/docklet/internal") } },
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
    proxy: serverModeBackend ? { "/wails": { target: serverModeBackend, ws: true } } : undefined,
  },
  build: {
    rolldownOptions: {
      output: {
        codeSplitting: {
          groups: [
            { name: "xterm", test: /node_modules[\/]@xterm/, priority: 30 },
            { name: "tanstack", test: /node_modules[\/]@tanstack/, priority: 20 },
            { name: "router", test: /node_modules[\/](react-router|react-router-dom|@remix-run)[\/]/, priority: 20 },
            { name: "radix-ui", test: /node_modules[\/](@radix-ui|radix-ui|cmdk|vaul|sonner|lucide-react)[\/]/, priority: 10 },
            { name: "react", test: /node_modules[\/](react|react-dom|scheduler)[\/]/, priority: 10 },
          ],
        },
      },
    },
  },
  plugins: [react(), tailwindcss(), wails("./bindings")],
  test: {
    coverage: {
      provider: "v8",
      // Count every source file, also the ones no test imports; otherwise the percentage only
      // describes the files that already have tests.
      include: ["src/**/*.{ts,tsx}"],
      exclude: ["src/**/*.test.ts", "src/**/*.d.ts"],
      reporter: ["text-summary", "html"],
      reportsDirectory: "coverage",
    },
  },
});
