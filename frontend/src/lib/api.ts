// Capa de comunicación con el backend
// Todos los errores de red se manejan aquí para que los componentes no necesiten try/catch

const BASE_URL = 'http://localhost:8080/v1';

export interface ApiSale {
	id: string;
	merchant_id: string;
	status: string;
	total: number;
	items: { product_id: string; quantity: number; unit_price: number }[];
	created_at: string;
}

export interface ApiProduct {
	id: string;
	name: string;
	price: number;
	stock: number;
}

export interface DashboardAggregate {
	merchant_id: string;
	total_sales: number;
	total_revenue: number;
	approved: number;
	declined: number;
	unknown: number;
	updated_at: string;
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
	const res = await fetch(`${BASE_URL}${path}`, {
		...options,
		headers: { 'Content-Type': 'application/json', ...options?.headers }
	});
	if (!res.ok) {
		const err = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(err.error ?? res.statusText);
	}
	return res.json();
}

export const api = {
	getProducts: () => request<ApiProduct[]>('/products'),

	createProduct: (data: { name: string; price: number; stock: number }) =>
		request<ApiProduct>('/products', { method: 'POST', body: JSON.stringify(data) }),

	createSale: (items: { product_id: string; quantity: number }[]) =>
		request<ApiSale>('/sales', { method: 'POST', body: JSON.stringify({ items }) }),

	getSale: (id: string) => request<ApiSale>(`/sales/${id}`),

	paySale: (
		id: string,
		data: { method: string; scenario: string },
		idempotencyKey: string
	) =>
		request<ApiSale>(`/sales/${id}/pay`, {
			method: 'POST',
			body: JSON.stringify(data),
			headers: { 'X-Idempotency-Key': idempotencyKey }
		}),

	getDashboard: () => request<DashboardAggregate>('/dashboard/today'),

	// SSE — devuelve el EventSource para que el componente lo cierre al desmontar
	streamDashboard: (onMessage: (data: DashboardAggregate) => void): EventSource => {
		const es = new EventSource(`${BASE_URL}/dashboard/stream`);
		es.onmessage = (e) => {
			try {
				onMessage(JSON.parse(e.data));
			} catch {
				// ignorar mensajes malformados
			}
		};
		return es;
	}
};
