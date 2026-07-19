import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const proxyPaths = [
  '/authenticate',
  '/logout',
  '/user-info',
  '/available-devices',
  '/device',
  '/devices',
  '/provider',
  '/admin',
  '/workspaces',
  '/client-credentials',
  '/custom-actions',
  '/health',
  '/ice-config',
  '/appium-logs',
  '/oauth',
  '/grid',
  '/provider-update',
]

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: Object.fromEntries(
      proxyPaths.map((path) => [
        path,
        { target: 'http://localhost:10000', changeOrigin: true },
      ]),
    ),
  },
})
