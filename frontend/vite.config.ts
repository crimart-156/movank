import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import { VitePWA } from 'vite-plugin-pwa';

export default defineConfig({
	plugins: [
		sveltekit({
			compilerOptions: { runes: true },
			adapter: adapter({ fallback: 'index.html' })
		}),
		VitePWA({
			registerType: 'autoUpdate',
			// App shell precacheado — funciona offline desde el primer arranque
			// No usamos Network First porque requiere red para arrancar
			// Usamos Cache First para el app shell
			workbox: {
				globPatterns: ['**/*.{js,css,html,ico,png,svg,woff2}'],
				runtimeCaching: [
					{
						// API — Network First con fallback a cache
						urlPattern: /^http:\/\/localhost:8080\/v1\/.*/,
						handler: 'NetworkFirst',
						options: {
							cacheName: 'api-cache',
							networkTimeoutSeconds: 5,
							expiration: { maxEntries: 50, maxAgeSeconds: 300 }
						}
					}
				]
			},
			manifest: {
				name: 'Movank Sales',
				short_name: 'Movank',
				description: 'Sistema de ventas offline-first',
				theme_color: '#1a1a2e',
				background_color: '#ffffff',
				display: 'standalone',
				start_url: '/',
				icons: [
					{ src: '/icon-192.png', sizes: '192x192', type: 'image/png' },
					{ src: '/icon-512.png', sizes: '512x512', type: 'image/png' }
				]
			}
		})
	]
});
