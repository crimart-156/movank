<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';

	let { children } = $props();
	let online = $state(true);

	onMount(() => {
		online = navigator.onLine;
		window.addEventListener('online', () => (online = true));
		window.addEventListener('offline', () => (online = false));
	});
</script>

<div class="app">
	<nav>
		<a href="/" class:active={$page.url.pathname === '/'}>Catálogo</a>
		<a href="/cart" class:active={$page.url.pathname === '/cart'}>Carrito</a>
		<a href="/dashboard" class:active={$page.url.pathname === '/dashboard'}>Dashboard</a>
		<span class="status" class:offline={!online}>
			{online ? '🟢 Online' : '🔴 Offline'}
		</span>
	</nav>

	<main>
		{@render children()}
	</main>
</div>

<style>
	.app { font-family: system-ui, sans-serif; max-width: 1200px; margin: 0 auto; padding: 0 1rem; }
	nav { display: flex; gap: 1.5rem; align-items: center; padding: 1rem 0; border-bottom: 1px solid #e2e8f0; }
	nav a { text-decoration: none; color: #64748b; font-weight: 500; }
	nav a.active { color: #1a1a2e; border-bottom: 2px solid #1a1a2e; }
	.status { margin-left: auto; font-size: 0.85rem; }
	.status.offline { color: #dc2626; }
	main { padding: 1.5rem 0; }
</style>
