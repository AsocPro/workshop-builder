import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'
import tailwindcss from '@tailwindcss/vite'

const backendURL = process.env.BACKEND_URL ?? 'http://localhost:8080'

export default defineConfig({
  plugins: [
    tailwindcss(),
    svelte(),
  ],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: backendURL,
        changeOrigin: true,
      },
      '/ws': {
        target: backendURL.replace(/^http/, 'ws'),
        ws: true,
        changeOrigin: true,
      },
      '/ttyd': {
        target: backendURL,
        changeOrigin: true,
        ws: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
