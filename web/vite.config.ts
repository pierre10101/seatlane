import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// In dev, Vite serves the UI and forwards /api (and the dev clock) to the
// Go server, so the session cookie and every rule stay on the Go side.
const api = process.env.SEATLANE_API ?? 'http://localhost:8080'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: { alias: { '@': new URL('./src', import.meta.url).pathname } },
  build: {
    rolldownOptions: {
      output: {
        codeSplitting: {
          groups: [
            { name: 'react', test: /node_modules[\\/](react|react-dom|react-router|react-router-dom|scheduler)[\\/]/ },
            { name: 'motion', test: /node_modules[\\/](framer-motion|motion-dom|motion-utils)[\\/]/ },
            { name: 'ui', test: /node_modules[\\/](radix-ui|@radix-ui|sonner|lucide-react)[\\/]/ },
          ],
        },
      },
    },
  },
  server: {
    proxy: {
      '/api': { target: api, changeOrigin: false },
      '/__dev': { target: api, changeOrigin: false },
    },
  },
})
