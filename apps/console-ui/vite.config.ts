import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

const controlPlaneOrigin = process.env.OPL_CONTROL_PLANE_API_ORIGIN || "http://127.0.0.1:8080";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/api": controlPlaneOrigin
    }
  }
});
