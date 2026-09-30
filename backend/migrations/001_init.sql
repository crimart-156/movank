-- Enable UUID generation
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Products
CREATE TABLE IF NOT EXISTS products (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    price       NUMERIC(12, 2) NOT NULL CHECK (price > 0),
    stock       INTEGER NOT NULL DEFAULT 0 CHECK (stock >= 0),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Sales
CREATE TABLE IF NOT EXISTS sales (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Multi-tenant: merchant_id siempre viene del token, nunca del body
    merchant_id     UUID NOT NULL,
    status          TEXT NOT NULL DEFAULT 'PENDING'
                        CHECK (status IN ('PENDING', 'APPROVED', 'DECLINED', 'UNKNOWN', 'PENDING_SYNC')),
    total           NUMERIC(12, 2) NOT NULL CHECK (total > 0),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Sale items
CREATE TABLE IF NOT EXISTS sale_items (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sale_id     UUID NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
    product_id  UUID NOT NULL REFERENCES products(id),
    quantity    INTEGER NOT NULL CHECK (quantity > 0),
    unit_price  NUMERIC(12, 2) NOT NULL CHECK (unit_price > 0)
);

-- Payments — guarda cada intento con su idempotency_key para evitar cobros duplicados
CREATE TABLE IF NOT EXISTS payments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sale_id         UUID NOT NULL REFERENCES sales(id),
    idempotency_key TEXT NOT NULL UNIQUE,   -- evita duplicar el cobro si se reintenta
    method          TEXT NOT NULL,           -- CARD, CASH, etc.
    status          TEXT NOT NULL            -- APPROVED, DECLINED, UNKNOWN
                        CHECK (status IN ('APPROVED', 'DECLINED', 'UNKNOWN')),
    scenario        TEXT NOT NULL,           -- APPROVED, DECLINED, TIMEOUT (para testing)
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Outbox — escribe en la misma transacción que la venta
-- El worker interno lo lee y empuja por SSE
CREATE TABLE IF NOT EXISTS outbox (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL,
    event_type  TEXT NOT NULL,               -- sale.paid, sale.declined, etc.
    payload     JSONB NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'published')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Índices útiles
CREATE INDEX IF NOT EXISTS idx_sales_merchant ON sales(merchant_id);
CREATE INDEX IF NOT EXISTS idx_sales_created ON sales(created_at);
CREATE INDEX IF NOT EXISTS idx_outbox_status ON outbox(status, created_at);
CREATE INDEX IF NOT EXISTS idx_payments_sale ON payments(sale_id);

-- Función que dispara NOTIFY cuando se inserta en outbox
-- Así el worker Go usa LISTEN/NOTIFY en lugar de polling
CREATE OR REPLACE FUNCTION notify_outbox()
RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('outbox_event', NEW.id::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE TRIGGER outbox_insert_trigger
    AFTER INSERT ON outbox
    FOR EACH ROW EXECUTE FUNCTION notify_outbox();
