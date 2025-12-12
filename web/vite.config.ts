import { defineConfig } from "vite";
import react from "@vitejs/plugin-react-swc";

// dev 模式下把 /api 代理到 Go 后端
export default defineConfig({
    plugins: [react()],
    server: {
        port: 5173,
        proxy: {
            "/api": {
                target: "http://127.0.0.1:8901",
                changeOrigin: true
            }
        }
    },
    build: {
        outDir: "dist"
    }
});
