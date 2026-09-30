<script lang="ts">
	import { page } from '$app/stores';

	const id = $derived($page.url.searchParams.get('id') ?? '');
	const status = $derived($page.url.searchParams.get('status') ?? '');

	const statusConfig: Record<string, { icon: string; label: string; color: string; message: string }> = {
		APPROVED:     { icon: '✅', label: 'Aprobado',          color: '#16a34a', message: 'Tu pago fue procesado exitosamente.' },
		DECLINED:     { icon: '❌', label: 'Rechazado',         color: '#dc2626', message: 'El pago fue rechazado por el proveedor.' },
		UNKNOWN:      { icon: '⚠️', label: 'Estado desconocido', color: '#d97706', message: 'No pudimos confirmar el estado del pago. Será verificado pronto.' },
		PENDING_SYNC: { icon: '🔄', label: 'Pendiente de sync', color: '#2563eb', message: 'Guardado localmente. Se sincronizará cuando recuperes la red.' },
	};

	const config = $derived(statusConfig[status] ?? statusConfig['UNKNOWN']);
</script>

<div class="receipt">
	<div class="icon">{config.icon}</div>
	<h1 style="color: {config.color}">{config.label}</h1>
	<p>{config.message}</p>

	{#if status === 'PENDING_SYNC'}
		<p class="hint">Puedes cerrar la app — la venta no se perderá.</p>
	{/if}

	<p class="id">ID: <code>{id}</code></p>

	<div class="actions">
		<a href="/" class="btn primary">Nueva venta</a>
		<a href="/dashboard" class="btn secondary">Ver dashboard</a>
	</div>
</div>

<style>
	.receipt { max-width: 400px; margin: 3rem auto; text-align: center; padding: 2rem; border: 1px solid #e2e8f0; border-radius: 12px; }
	.icon { font-size: 3rem; margin-bottom: 0.5rem; }
	h1 { font-size: 1.5rem; margin: 0 0 0.75rem; }
	p { color: #475569; margin: 0.5rem 0; }
	.hint { font-size: 0.85rem; color: #64748b; }
	.id { font-size: 0.8rem; margin-top: 1.5rem; color: #94a3b8; }
	code { font-size: 0.75rem; word-break: break-all; }
	.actions { display: flex; gap: 1rem; justify-content: center; margin-top: 1.5rem; }
	.btn { padding: 0.6rem 1.2rem; border-radius: 6px; text-decoration: none; font-weight: 500; font-size: 0.9rem; }
	.btn.primary { background: #1a1a2e; color: white; }
	.btn.secondary { background: #f1f5f9; color: #1a1a2e; }
</style>
