import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "vitest/config";

// `make run` serves web/dist; in dev the engine runs on :8080.
export default defineConfig({
  plugins: [tailwindcss()],
  build: { outDir: "dist" },
  server: {
    proxy: {
      "/api": "http://localhost:8080",
      "/ws": { target: "ws://localhost:8080", ws: true },
    },
  },
  test: { include: ["src/**/*.test.ts"] },
});
