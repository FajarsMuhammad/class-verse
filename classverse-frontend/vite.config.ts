import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'

const backend = 'http://localhost:8080'

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    vue(),
    vueDevTools(),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    // Proxy to the Go server so the browser sees one origin: the refresh-token
    // cookie is SameSite=Strict and no CORS is needed.
    proxy: {
      '/api': backend,
      '/ws': { target: backend, ws: true },
      '/healthz': backend,
    },
  },
})
