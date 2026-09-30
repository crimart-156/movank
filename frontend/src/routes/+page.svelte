<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import { getProducts, saveProducts, saveCartItem, getCart, type Product, type CartItem } from '$lib/db';

	let products = $state<Product[]>([]);
	let cart = $state<CartItem[]>([]);
	let loading = $state(true);
	let error = $state('');
	let addedId = $state('');

	onMount(async () => {
		cart = await getCart();

		if (navigator.onLine) {
			try {
				// Online: el backend es la fuente de verdad
				const backendProducts = await api.getProducts();

				if (backendProducts.length === 0) {
					// Primera vez: crear productos demo en el backend
					const demos = [
						{ name: 'Café Americano', price: 3500, stock: 50 },
						{ name: 'Cappuccino', price: 4500, stock: 30 },
						{ name: 'Empanada', price: 2500, stock: 20 },
						{ name: 'Agua 500ml', price: 1500, stock: 100 }
					];
					const created = await Promise.all(demos.map(d => api.createProduct(d)));
					products = created.map(p => ({ id: p.id, name: p.name, price: p.price, stock: p.stock }));
				} else {
					products = backendProducts.map(p => ({ id: p.id, name: p.name, price: p.price, stock: p.stock }));
				}

				// Sincronizar IndexedDB con lo que tiene el backend (para uso offline)
				await saveProducts(products);
			} catch {
				// Red falló — usar IndexedDB como fallback
				products = await getProducts();
			}
		} else {
			// Offline: siempre desde IndexedDB
			products = await getProducts();
		}

		loading = false;
	});


	async function addToCart(product: Product) {
		const existing = cart.find(i => i.product_id === product.id);
		const item: CartItem = existing
			? { ...existing, quantity: existing.quantity + 1 }
			: { product_id: product.id, name: product.name, unit_price: product.price, quantity: 1 };

		await saveCartItem(item);
		cart = await getCart();
		addedId = product.id;
		setTimeout(() => (addedId = ''), 1000);
	}

	function cartCount(productId: string) {
		return cart.find(i => i.product_id === productId)?.quantity ?? 0;
	}
</script>

<div>
	<h1>Catálogo</h1>
	{#if !navigator?.onLine}
		<p class="banner offline">Modo offline — mostrando catálogo guardado</p>
	{/if}

	{#if loading}
		<p>Cargando...</p>
	{:else if error}
		<p class="error">{error}</p>
	{:else}
		<!-- Grid responsivo con CSS Grid auto-fit/minmax
			 Se adapta según el ancho del contenedor, no del viewport -->
		<div class="grid">
			{#each products as product (product.id)}
				<div class="card">
					<h3>{product.name}</h3>
					<p class="price">${product.price.toLocaleString('es-CO')}</p>
					<p class="stock">Stock: {product.stock}</p>
					<div class="actions">
						{#if cartCount(product.id) > 0}
							<span class="badge">{cartCount(product.id)} en carrito</span>
						{/if}
						<button
							class="btn primary"
							class:added={addedId === product.id}
							onclick={() => addToCart(product)}
						>
							{addedId === product.id ? '✓ Agregado' : 'Agregar'}
						</button>
					</div>
				</div>
			{/each}
		</div>

		{#if cart.length > 0}
			<div class="cart-bar">
				<span>{cart.reduce((s, i) => s + i.quantity, 0)} productos en carrito</span>
				<a href="/cart" class="btn primary">Ver carrito →</a>
			</div>
		{/if}
	{/if}
</div>

<style>
	h1 { font-size: 1.5rem; margin-bottom: 1rem; }

	/* Container queries para el grid — responde al ancho del contenedor */
	.grid {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
		gap: 1rem;
	}

	.card {
		border: 1px solid #e2e8f0;
		border-radius: 8px;
		padding: 1rem;
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}
	.card h3 { margin: 0; font-size: 1rem; }
	.price { font-size: 1.2rem; font-weight: 700; color: #1a1a2e; margin: 0; }
	.stock { font-size: 0.8rem; color: #64748b; margin: 0; }
	.actions { display: flex; align-items: center; gap: 0.5rem; margin-top: auto; }
	.badge { font-size: 0.75rem; background: #e2e8f0; padding: 2px 6px; border-radius: 4px; }

	.btn { padding: 0.5rem 1rem; border: none; border-radius: 6px; cursor: pointer; font-size: 0.9rem; text-decoration: none; }
	.btn.primary { background: #1a1a2e; color: white; }
	.btn.primary.added { background: #16a34a; }

	.cart-bar {
		position: fixed; bottom: 1rem; left: 50%; transform: translateX(-50%);
		background: white; border: 1px solid #e2e8f0; border-radius: 8px;
		padding: 0.75rem 1.5rem; display: flex; gap: 1rem; align-items: center;
		box-shadow: 0 4px 12px rgba(0,0,0,0.1);
	}

	.banner { padding: 0.5rem 1rem; border-radius: 6px; margin-bottom: 1rem; font-size: 0.9rem; }
	.offline { background: #fef3c7; color: #92400e; }
	.error { color: #dc2626; }
</style>
