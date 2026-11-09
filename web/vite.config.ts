import { defineConfig } from 'vite';
import { fileURLToPath } from 'node:url';

export default defineConfig({
  root: fileURLToPath(new URL('.', import.meta.url)),
  server: { host: '127.0.0.1', proxy: { '/api': { target: 'http://127.0.0.1:8080', changeOrigin: false } } },
  build: { outDir: 'dist', emptyOutDir: true },
});
