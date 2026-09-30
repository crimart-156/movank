<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import {
		getCart, clearCart, saveSale, removeCartItem, saveCartItem,
		type CartItem, type Sale
	} from '$lib/db';

	let cart = $state<CartItem[]>([]);
	let loading = $state(false);
	let error = $state('');

	const total = $derived(cart.reduce((s, i) => s + i.unit_price * i.quantity, 0));

	onMount(async () => {
		cart = await getCart();
	});

	async function updateQty(item: CartItem, delta: number) {
		const newQty = item.quantity + delta;
		if (newQty <= 0) {
			await removeCartItem(item.product_id);
		} else {
			await saveCartItem({ ...item, quantity: newQty });
		}
		cart = await getCart();
	}

	async function pay() {
		if (cart.length === 0) return;
		loading = true;
		error = '';

		// Generar idempotency key antes del intento — si falla la red, usamos el mismo key al reintentar
		const idempotencyKey = crypto.randomUUID();

		if (!navigator.onLine) {
			// Offline: guardar en IndexedDB como PENDING_SYNC
			const offlineSale: Sale = {
				id: crypto.randomUUID(),
				merchant_id: '00000000-0000-0000-0000-000000000001',
				status: 'PENDING_SYNC',
				total,
				items: cart,
				created_at: new Date().toISOString(),
				idempotency_key: idempotencyKey
			};
			await saveSale(offlineSale);
			await clearCart();
			goto(`/receipt?id=${offlineSale.id}&status=PENDING_SYNC`);
			return;
		}

		try {
			// Online: crear la venta en el backend
			const sale = await api.createSale(
				cart.map(i => ({ product_id: i.product_id, quantity: i.quantity }))
			);

			// Procesar pago
			const paid = await api.paySale(
				sale.id,
				{ method: 'CARD', scenario: 'APPROVED' },
				idempotencyKey
			);

			// Guardar en IndexedDB para historial offline
			await saveSale({
				id: paid.id,
				merchant_id: paid.merchant_id,
				status: paid.status as any,
				total: paid.total,
				items: cart,
				created_at: paid.created_at,
				idempotency_key: idempotencyKey
			});

			await clearCart();
			goto(`/receipt?id=${paid.id}&status=${paid.status}`);
		} catch (e: any) {
			error = e.message ?? 'Error procesando el pago';
		} finally {
			loading = false;
		}
	}
</script>

<div>
	<h1>Carrito</h1>

	{#if cart.length === 0}
		<p>Tu carrito está vacío. <a href="/">Ver catálogo</a></p>
	{:else}
		<div class="items">
			{#each cart as item (item.product_id)}
				<div class="item">
					<span class="name">{item.name}</span>
					<div class="qty">
						<button onclick={() => updateQty(item, -1)}>−</button>
						<span>{item.quantity}</span>
						<button onclick={() => updateQty(item, +1)}>+</button>
					</div>
					<span class="subtotal">${(item.unit_price * item.quantity).toLocaleString('es-CO')}</span>
					<button class="remove" onclick={() => updateQty(item, -item.quantity)}>✕</button>
				</div>
			{/each}
		</div>

		<div class="summary">
			<span class="total-label">Total</span>
			<span class="total-value">${total.toLocaleString('es-CO')}</span>
		</div>

		{#if !navigator?.onLine}
			<p class="banner offline">Offline — el pago se guardará y sincronizará cuando recuperes la red</p>
		{/if}

		{#if error}
			<p class="error">{error}</p>
		{/if}

		<div class="actions">
			<a href="/" class="btn danger">← Seguir comprando</a>
			<button class="btn primary" onclick={pay} disabled={loading}>
				{loading ? 'Procesando...' : navigator?.onLine ? 'Pagar ahora' : 'Guardar y sincronizar después'}
			</button>
		</div>
	{/if}
</div>

<style>
	h1 { font-size: 1.5rem; margin-bottom: 1rem; }
	.items { display: flex; flex-direction: column; gap: 0.75rem; margin-bottom: 1.5rem; }
	.item {
		display: flex; align-items: center; gap: 1rem;
		padding: 0.75rem; border: 1px solid #e2e8f0; border-radius: 8px;
	}
	.name { flex: 1; font-weight: 500; }
	.qty { display: flex; align-items: center; gap: 0.5rem; }
	.qty button { width: 28px; height: 28px; border: 1px solid #e2e8f0; border-radius: 4px; background: white; cursor: pointer; }
	.subtotal { min-width: 80px; text-align: right; font-weight: 600; }
	.remove { background: none; border: none; color: #dc2626; cursor: pointer; font-size: 1rem; }
	.summary { display: flex; justify-content: space-between; padding: 1rem 0; border-top: 2px solid #1a1a2e; margin-bottom: 1.5rem; }
	.total-label { font-size: 1.1rem; font-weight: 600; }
	.total-value { font-size: 1.4rem; font-weight: 700; color: #1a1a2e; }
	.actions { display: flex; gap: 1rem; }
	.btn { padding: 0.75rem 1.5rem; border: none; border-radius: 6px; cursor: pointer; font-size: 1rem; text-decoration: none; font-weight: 500; }
	.btn.primary { background: #1a1a2e; color: white; }
	.btn.primary:disabled { opacity: 0.6; cursor: not-allowed; }
	.btn.danger { background: #fee2e2; color: #dc2626; }
	.banner { padding: 0.75rem 1rem; border-radius: 6px; margin-bottom: 1rem; font-size: 0.9rem; }
	.offline { background: #fef3c7; color: #92400e; }
	.error { color: #dc2626; margin-bottom: 1rem; }
</style>
