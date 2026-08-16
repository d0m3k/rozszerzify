import { execSync } from 'child_process';
import { defineConfig } from 'vite';
import preact from '@preact/preset-vite';
import { VitePWA } from 'vite-plugin-pwa';

const BUILD_HASH = (process.env.GITHUB_SHA?.slice(0, 7))
  || (() => {
    try {
      return execSync('git rev-parse --short HEAD', { encoding: 'utf-8' }).trim();
    } catch {
      return 'dev';
    }
  })();

export default defineConfig({
  define: {
    __BUILD_HASH__: JSON.stringify(BUILD_HASH),
  },
  plugins: [
    preact(),
    VitePWA({
      registerType: 'autoUpdate',
      injectRegister: 'auto',
      workbox: {
        // terser misbehaves on this platform (Android/Termux) — skip SW minification
        mode: 'development',
        globPatterns: ['**/*.{js,css,ico,png,svg,webmanifest}'],
        navigateFallbackDenylist: [/^\/api\//],
        navigateFallback: '/index.html',
        runtimeCaching: [
          {
            urlPattern: /^\/api\/.*/,
            handler: 'NetworkFirst',
            options: {
              cacheName: 'api-cache',
              expiration: { maxEntries: 60, maxAgeSeconds: 10 },
              networkTimeoutSeconds: 4,
            },
          },
        ],
      },
      manifest: {
        name: 'Rozszerzify — dieta niemowlaka',
        short_name: 'Rozszerzify',
        description: 'Śledź rozszerzanie diety: ile razy maluch próbował każde jedzenie i czy mu smakuje.',
        theme_color: '#FF8E72',
        background_color: '#FFF8F0',
        display: 'standalone',
        orientation: 'portrait',
        start_url: `/?v=${BUILD_HASH}`,
        scope: '/',
        icons: [
          { src: `/icon-192.png?v=${BUILD_HASH}`, sizes: '192x192', type: 'image/png', purpose: 'any' },
          { src: `/icon-512.png?v=${BUILD_HASH}`, sizes: '512x512', type: 'image/png', purpose: 'any' },
          { src: `/icon-192-maskable.png?v=${BUILD_HASH}`, sizes: '192x192', type: 'image/png', purpose: 'maskable' },
          { src: `/icon-512-maskable.png?v=${BUILD_HASH}`, sizes: '512x512', type: 'image/png', purpose: 'maskable' },
          { src: '/favicon.svg', sizes: 'any', type: 'image/svg+xml', purpose: 'any' },
        ],
      },
    }),
  ],
  build: {
    outDir: 'dist',
    sourcemap: false,
  },
  server: {
    host: '0.0.0.0',
    port: 5173,
    proxy: {
      '/api': 'http://127.0.0.1:8081',
    },
  },
});