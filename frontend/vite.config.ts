import path from "node:path";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import wails from "@wailsio/runtime/plugins/vite";

export default defineConfig({
  resolve: { alias: { "@": path.resolve(__dirname, "./src"), "@bindings": path.resolve(__dirname, "./bindings/docklet/internal") } },
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
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
});
