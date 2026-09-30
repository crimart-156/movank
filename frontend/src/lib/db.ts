// IndexedDB — persistencia local para catálogo, carrito y ventas pendientes
// No usamos localStorage porque tiene límite de 5MB y es síncrono

const DB_NAME = 'movank';
const DB_VERSION = 1;

export interface Product {
	id: string;
	name: string;
	price: number;
	stock: number;
}

export interface CartItem {
	product_id: string;
	name: string;
	unit_price: number;
	quantity: number;
}

export type SaleStatus = 'PENDING' | 'APPROVED' | 'DECLINED' | 'UNKNOWN' | 'PENDING_SYNC';

export interface Sale {
	id: string;
	merchant_id: string;
	status: SaleStatus;
	total: number;
	items: CartItem[];
	created_at: string;
	idempotency_key: string;
}

function openDB(): Promise<IDBDatabase> {
	return new Promise((resolve, reject) => {
		const req = indexedDB.open(DB_NAME, DB_VERSION);

		req.onupgradeneeded = (e) => {
			const db = (e.target as IDBOpenDBRequest).result;
			// Catálogo de productos
			if (!db.objectStoreNames.contains('products')) {
				db.createObjectStore('products', { keyPath: 'id' });
			}
			// Carrito actual — una sola entrada por merchant
			if (!db.objectStoreNames.contains('cart')) {
				db.createObjectStore('cart', { keyPath: 'product_id' });
			}
			// Cola de ventas pendientes de sincronizar
			if (!db.objectStoreNames.contains('sales')) {
				const store = db.createObjectStore('sales', { keyPath: 'id' });
				store.createIndex('status', 'status', { unique: false });
			}
		};

		req.onsuccess = () => resolve(req.result);
		req.onerror = () => reject(req.error);
	});
}

// --- Productos ---

export async function saveProducts(products: Product[]): Promise<void> {
	const db = await openDB();
	const tx = db.transaction('products', 'readwrite');
	const store = tx.objectStore('products');
	// Limpiar primero para no acumular duplicados
	store.clear();
	// Convertir a objeto plano — Svelte 5 usa proxies reactivos que IndexedDB no puede clonar
	for (const p of products) store.put({ id: p.id, name: p.name, price: p.price, stock: p.stock });
	return new Promise((res, rej) => {
		tx.oncomplete = () => res();
		tx.onerror = () => rej(tx.error);
	});
}

export async function getProducts(): Promise<Product[]> {
	const db = await openDB();
	const tx = db.transaction('products', 'readonly');
	const store = tx.objectStore('products');
	return new Promise((res, rej) => {
		const req = store.getAll();
		req.onsuccess = () => res(req.result);
		req.onerror = () => rej(req.error);
	});
}

// --- Carrito ---

export async function getCart(): Promise<CartItem[]> {
	const db = await openDB();
	const tx = db.transaction('cart', 'readonly');
	return new Promise((res, rej) => {
		const req = tx.objectStore('cart').getAll();
		req.onsuccess = () => res(req.result);
		req.onerror = () => rej(req.error);
	});
}

export async function saveCartItem(item: CartItem): Promise<void> {
	const db = await openDB();
	const tx = db.transaction('cart', 'readwrite');
	tx.objectStore('cart').put({ product_id: item.product_id, name: item.name, unit_price: item.unit_price, quantity: item.quantity });
	return new Promise((res, rej) => {
		tx.oncomplete = () => res();
		tx.onerror = () => rej(tx.error);
	});
}

export async function removeCartItem(product_id: string): Promise<void> {
	const db = await openDB();
	const tx = db.transaction('cart', 'readwrite');
	tx.objectStore('cart').delete(product_id);
	return new Promise((res, rej) => {
		tx.oncomplete = () => res();
		tx.onerror = () => rej(tx.error);
	});
}

export async function clearCart(): Promise<void> {
	const db = await openDB();
	const tx = db.transaction('cart', 'readwrite');
	tx.objectStore('cart').clear();
	return new Promise((res, rej) => {
		tx.oncomplete = () => res();
		tx.onerror = () => rej(tx.error);
	});
}

// --- Ventas ---

export async function saveSale(sale: Sale): Promise<void> {
	const db = await openDB();
	const tx = db.transaction('sales', 'readwrite');
	tx.objectStore('sales').put(JSON.parse(JSON.stringify(sale)));
	return new Promise((res, rej) => {
		tx.oncomplete = () => res();
		tx.onerror = () => rej(tx.error);
	});
}

export async function getPendingSales(): Promise<Sale[]> {
	const db = await openDB();
	const tx = db.transaction('sales', 'readonly');
	const store = tx.objectStore('sales');
	const index = store.index('status');
	return new Promise((res, rej) => {
		const req = index.getAll('PENDING_SYNC');
		req.onsuccess = () => res(req.result);
		req.onerror = () => rej(req.error);
	});
}

export async function updateSaleStatus(id: string, status: SaleStatus): Promise<void> {
	const db = await openDB();
	const tx = db.transaction('sales', 'readwrite');
	const store = tx.objectStore('sales');
	return new Promise((res, rej) => {
		const req = store.get(id);
		req.onsuccess = () => {
			const sale = req.result;
			if (sale) {
				sale.status = status;
				store.put(sale);
			}
			tx.oncomplete = () => res();
		};
		req.onerror = () => rej(req.error);
	});
}
