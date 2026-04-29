import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
// The request-ui is a public-facing SPA served at the root.
// In production it sits behind an nginx container on port 3002.
// In dev mode the Vite proxy forwards /api calls to Traefik (port 80).
export default defineConfig({
    plugins: [react()],
    server: {
        port: 3002,
        host: '0.0.0.0',
        proxy: {
            '/api': {
                target: 'http://localhost:80',
                changeOrigin: true,
            },
        },
    },
    build: {
        outDir: 'dist',
    },
});
