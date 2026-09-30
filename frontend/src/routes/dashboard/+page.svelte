<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { api, type DashboardAggregate } from '$lib/api';

	let data = $state<DashboardAggregate | null>(null);
	let loading = $state(true);
	let online = $state(true);
	let es: EventSource | null = null;

	onMount(async () => {
		online = navigator.onLine;
		window.addEventListener('online', handleOnline);
		window.addEventListener('offline', handleOffline);

		// Cargar estado inicial
		try {
			data = await api.getDashboard();
		} catch {
			// Si falla (offline), data queda null — mostramos el último cacheado
		}
		loading = false;

		// Suscribir al SSE si hay red
		if (navigator.onLine) {
			connectSSE();
		}

		// Iniciar Web Worker de sincronización
		startSyncWorker();
	});

	onDestroy(() => {
		es?.close();
		window.removeEventListener('online', handleOnline);
		window.removeEventListener('offline', handleOffline);
	});

	function connectSSE() {
		es?.close();
		// SSE actualiza el dashboard en tiempo real cuando llega una venta aprobada
		es = api.streamDashboard((newData) => {
			data = newData;
		});
	}

	function handleOnline() {
		online = true;
		connectSSE();
	}

	function handleOffline() {
		online = false;
		es?.close();
	}

	function startSyncWorker() {
		// Web Worker fuera del hilo principal — no bloquea la UI
		const worker = new Worker(new URL('$lib/sync.worker.ts', import.meta.url), { type: 'module' });

		worker.postMessage({ type: 'SYNC' });

		// Cuando una venta se sincroniza, refrescar el dashboard
		worker.onmessage = (e) => {
			if (e.data?.type === 'SALE_SYNCED') {
				api.getDashboard().then(d => (data = d)).catch(() => {});
			}
		};
	}
</script>

<div>
	<div class="header">
		<h1>Dashboard — Hoy</h1>
		<span class="badge" class:offline={!online}>
			{online ? '🟢 Tiempo real' : '🔴 Offline — último dato guardado'}
		</span>
	</div>

	{#if loading}
		<p>Cargando...</p>
	{:else if !data}
		<p class="empty">Sin datos disponibles.</p>
	{:else}
		<div class="grid">
			<div class="card">
				<span class="label">Total ventas</span>
				<span class="value">{data.total_sales}</span>
			</div>
			<div class="card">
				<span class="label">Ingresos</span>
				<span class="value">${data.total_revenue.toLocaleString('es-CO')}</span>
			</div>
			<div class="card approved">
				<span class="label">Aprobadas</span>
				<span class="value">{data.approved}</span>
			</div>
			<div class="card declined">
				<span class="label">Rechazadas</span>
				<span class="value">{data.declined}</span>
			</div>
			<div class="card unknown">
				<span class="label">Desconocidas</span>
				<span class="value">{data.unknown}</span>
				{#if data.unknown > 0}
					<span class="hint">Requieren reconciliación</span>
				{/if}
			</div>
		</div>
		<p class="updated">Actualizado: {new Date(data.updated_at).toLocaleTimeString('es-CO')}</p>
	{/if}
</div>

<style>
	.header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 1.5rem; }
	h1 { font-size: 1.5rem; margin: 0; }
	.badge { font-size: 0.85rem; padding: 4px 10px; border-radius: 20px; background: #dcfce7; color: #166534; }
	.badge.offline { background: #fee2e2; color: #991b1b; }

	.grid {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
		gap: 1rem;
		margin-bottom: 1rem;
	}

	.card {
		padding: 1.25rem; border: 1px solid #e2e8f0; border-radius: 10px;
		display: flex; flex-direction: column; gap: 0.25rem;
	}
	.label { font-size: 0.8rem; color: #64748b; text-transform: uppercase; letter-spacing: 0.05em; }
	.value { font-size: 2rem; font-weight: 700; color: #1a1a2e; }
	.hint { font-size: 0.75rem; color: #d97706; }

	.card.approved { border-color: #bbf7d0; background: #f0fdf4; }
	.card.approved .value { color: #16a34a; }
	.card.declined { border-color: #fecaca; background: #fff1f2; }
	.card.declined .value { color: #dc2626; }
	.card.unknown { border-color: #fde68a; background: #fffbeb; }
	.card.unknown .value { color: #d97706; }

	.updated { font-size: 0.8rem; color: #94a3b8; }
	.empty { color: #64748b; }
</style>
