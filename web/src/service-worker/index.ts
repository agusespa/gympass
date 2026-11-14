/**
 * Offline support: precache the app shell and build output, answer page
 * navigations from the network when possible (so an expired Cloudflare Access
 * session reaches the login page) and from the cached shell when offline.
 * /api is never intercepted.
 */
import { self } from '$app/service-worker';
import { assets, immutable } from '$app/manifest';
import { dev, version } from '$app/env';

const CACHE = `gym-${version}`;
const SHELL = new URL('./', self.registration.scope).href;
const PRECACHE = [
	SHELL,
	...immutable.map((f) => new URL(f.path, self.registration.scope).href),
	...assets.map((f) => new URL(f.path, self.registration.scope).href)
];

self.addEventListener('install', (event) => {
	event.waitUntil(
		(async () => {
			const cache = await caches.open(CACHE);
			await cache.addAll(PRECACHE);
			await self.skipWaiting();
		})()
	);
});

self.addEventListener('activate', (event) => {
	event.waitUntil(
		(async () => {
			for (const key of await caches.keys()) {
				if (key !== CACHE) await caches.delete(key);
			}
			await self.clients.claim();
		})()
	);
});

self.addEventListener('fetch', (event) => {
	// Never serve cached modules to the dev server.
	if (dev) return;
	const req = event.request;
	if (req.method !== 'GET') return;
	const url = new URL(req.url);
	if (url.origin !== self.location.origin || url.pathname.startsWith('/api/')) return;

	if (req.mode === 'navigate') {
		event.respondWith(
			(async () => {
				try {
					// Pass redirects (e.g. to the Access login) straight to the browser.
					return await fetch(req);
				} catch {
					const cached = await caches.match(SHELL);
					return cached ?? Response.error();
				}
			})()
		);
		return;
	}

	event.respondWith(
		(async () => {
			const cached = await caches.match(req);
			if (cached) return cached;
			const res = await fetch(req);
			if (res.ok && res.type === 'basic') {
				const cache = await caches.open(CACHE);
				await cache.put(req, res.clone());
			}
			return res;
		})()
	);
});
