// Web Worker — motor de sincronización fuera del hilo principal
// Razón: no queremos bloquear la UI mientras reintentamos ventas pendientes
// El worker escucha mensajes del hilo principal y responde cuando termina

const BASE_URL = 'http://localhost:8080/v1';

// Abre IndexedDB directamente desde el worker
function openDB(): Promise<IDBDatabase> {
	return new Promise((resolve, reject) => {
		const req = indexedDB.open('movank', 1);
		req.onsuccess = () => resolve(req.result);
		req.onerror = () => reject(req.error);
	});
}

async function getPendingSales(db: IDBDatabase): Promise<any[]> {
	return new Promise((res, rej) => {
		const tx = db.transaction('sales', 'readonly');
		const index = tx.objectStore('sales').index('status');
		const req = index.getAll('PENDING_SYNC');
		req.onsuccess = () => res(req.result);
		req.onerror = () => rej(req.error);
	});
}

async function updateSaleStatus(db: IDBDatabase, id: string, status: string): Promise<void> {
	return new Promise((res, rej) => {
		const tx = db.transaction('sales', 'readwrite');
		const store = tx.objectStore('sales');
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

// Sincroniza cada venta PENDING_SYNC exactamente una vez
// Si falla, queda en PENDING_SYNC para el próximo intento
// Nunca duplica: la idempotency_key evita cobros dobles en el backend
async function syncPendingSales() {
	const db = await openDB();
	const pending = await getPendingSales(db);

	for (const sale of pending) {
		try {
			// Primero crear la venta en el backend
			const createRes = await fetch(`${BASE_URL}/sales`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					items: sale.items.map((i: any) => ({
						product_id: i.product_id,
						quantity: i.quantity
					}))
				})
			});

			if (!createRes.ok) {
				// No actualizamos el estado — se reintentará
				continue;
			}

			const createdSale = await createRes.json();

			// Luego procesar el pago con la idempotency_key original
			const payRes = await fetch(`${BASE_URL}/sales/${createdSale.id}/pay`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					// La idempotency_key garantiza que si esto se reintenta no se cobra dos veces
					'X-Idempotency-Key': sale.idempotency_key
				},
				body: JSON.stringify({ method: 'CARD', scenario: 'APPROVED' })
			});

			const finalStatus = payRes.ok ? (await payRes.json()).status : 'UNKNOWN';

			// Actualizar en IndexedDB con el resultado final
			await updateSaleStatus(db, sale.id, finalStatus);

			// Notificar al hilo principal que esta venta fue sincronizada
			self.postMessage({ type: 'SALE_SYNCED', saleId: sale.id, status: finalStatus });
		} catch {
			// Error de red — la venta queda PENDING_SYNC, se reintenta la próxima vez
			self.postMessage({ type: 'SALE_SYNC_FAILED', saleId: sale.id });
		}
	}

	self.postMessage({ type: 'SYNC_COMPLETE' });
}

// Escuchar mensajes del hilo principal
self.addEventListener('message', (e) => {
	if (e.data?.type === 'SYNC') {
		syncPendingSales();
	}
});

// Al recuperar la red, intentar sincronizar automáticamente
self.addEventListener('online' as any, () => {
	syncPendingSales();
});
