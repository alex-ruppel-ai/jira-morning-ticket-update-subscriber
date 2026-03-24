import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const backendPort = process.env.PORT || 8081;

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  server: {
    host: '0.0.0.0',
    watch: {
      usePolling: true,
      interval: 1000,
      ignored: ['**/node_modules/**', '**/.git/**'],
    },
    proxy: {
      '/api': {
        target: `http://localhost:${backendPort}`,
        changeOrigin: true,
        onError(err, req, res) {
          res.writeHead(503, { 'Content-Type': 'application/json' })
          res.end(JSON.stringify({
            status: 'building',
            message: 'Backend is starting, please wait...',
          }))
        },
      },
      '/slack': {
        target: `http://localhost:${backendPort}`,
        changeOrigin: true,
        onError(err, req, res) {
          res.writeHead(503, { 'Content-Type': 'application/json' })
          res.end(JSON.stringify({
            status: 'building',
            message: 'Backend is starting, please wait...',
          }))
        },
      }
    },
    allowedHosts: true,
  },
  // Middleware to allow iframe embedding
  configureServer(server) {
    server.middlewares.use((req, res, next) => {
      // Remove X-Frame-Options header to allow embedding in iframes
      res.removeHeader('X-Frame-Options')
      next()
    })
  }
})

