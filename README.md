# Movank — Sales API Full Stack

Sistema de ventas offline-first con backend en Go y frontend en SvelteKit.

---

## Stack

| Capa | Tecnología | Por qué |
|------|-----------|---------|
| Backend | Go + Chi | Concurrencia nativa con goroutines, ideal para el outbox worker y SSE |
| Base de datos | PostgreSQL 16 | LISTEN/NOTIFY nativo para el patrón outbox sin polling |
| Cache | Dragonfly | Compatible con Redis, más eficiente en memoria, mismo API |
| Frontend | SvelteKit + Vite | Compilado a JS puro, soporte PWA real con adapter-static |
| PWA | vite-plugin-pwa + Workbox | App shell precacheado, funciona offline desde el primer arranque |

---

## Requisitos

- [Docker](https://docs.docker.com/get-docker/) y Docker Compose
- [Go 1.27+](https://go.dev/dl/) (solo para desarrollo local)
- [Node.js 20+](https://nodejs.org/) (solo para desarrollo local)

---

## Levantar el proyecto completo

**Linux / macOS**
```bash
# 1. Clonar el repositorio
git clone https://github.com/crimart-156/movank.git
cd movank

# 2. Copiar variables de entorno
cp .env.example .env

# 3. Levantar infraestructura + backend (--build construye la imagen Go)
docker compose up -d --build

# 4. Verificar que los 3 servicios están corriendo
docker compose ps
```

**Windows (PowerShell)**
```powershell
# 1. Clonar el repositorio
git clone https://github.com/crimart-156/movank.git
cd movank

# 2. Copiar variables de entorno
Copy-Item .env.example .env

# 3. Levantar infraestructura + backend (--build construye la imagen Go)
docker compose up -d --build

# 4. Verificar que los 3 servicios están corriendo
docker compose ps
```

El backend queda disponible en `http://localhost:8080`.

### Levantar el frontend (desarrollo)

**Linux / macOS**
```bash
cd frontend
npm install
npm run dev
```

**Windows (PowerShell)**
```powershell
cd frontend
npm install --legacy-peer-deps
npm run dev
```

El frontend queda disponible en `http://localhost:5173`.

### Build de producción del frontend

```bash
cd frontend
npm run build
# Los archivos estáticos quedan en frontend/build/
```

---

## Probar los endpoints

### Linux / macOS (curl)

```bash
# Crear un producto
curl -X POST http://localhost:8080/v1/products \
  -H "Content-Type: application/json" \
  -d '{"name": "Café Americano", "price": 3500, "stock": 50}'

# Crear una venta (reemplazar UUID_PRODUCTO con el id devuelto arriba)
curl -X POST http://localhost:8080/v1/sales \
  -H "Content-Type: application/json" \
  -d '{"items": [{"product_id": "<UUID_PRODUCTO>", "quantity": 2}]}'

# Procesar un pago APPROVED (reemplazar UUID_VENTA)
curl -X POST http://localhost:8080/v1/sales/<UUID_VENTA>/pay \
  -H "Content-Type: application/json" \
  -H "X-Idempotency-Key: test-key-001" \
  -d '{"method": "CARD", "scenario": "APPROVED"}'

# Probar idempotencia — mismo key, no duplica el cobro
curl -X POST http://localhost:8080/v1/sales/<UUID_VENTA>/pay \
  -H "Content-Type: application/json" \
  -H "X-Idempotency-Key: test-key-001" \
  -d '{"method": "CARD", "scenario": "APPROVED"}'

# Probar TIMEOUT → debe quedar UNKNOWN, nunca DECLINED
curl -X POST http://localhost:8080/v1/sales/<UUID_VENTA>/pay \
  -H "Content-Type: application/json" \
  -H "X-Idempotency-Key: test-key-002" \
  -d '{"method": "CARD", "scenario": "TIMEOUT"}'

# Dashboard snapshot
curl http://localhost:8080/v1/dashboard/today

# Dashboard SSE en tiempo real
curl -N http://localhost:8080/v1/dashboard/stream
```

### Windows (PowerShell)

```powershell
# Crear un producto
Invoke-WebRequest -Uri "http://localhost:8080/v1/products" -Method POST `
  -Headers @{"Content-Type"="application/json"} `
  -Body '{"name":"Cafe Americano","price":3500,"stock":50}' | Select-Object -ExpandProperty Content

# Crear una venta (reemplazar UUID_PRODUCTO con el id devuelto arriba)
Invoke-WebRequest -Uri "http://localhost:8080/v1/sales" -Method POST `
  -Headers @{"Content-Type"="application/json"} `
  -Body '{"items":[{"product_id":"<UUID_PRODUCTO>","quantity":2}]}' | Select-Object -ExpandProperty Content

# Procesar un pago APPROVED (reemplazar UUID_VENTA)
Invoke-WebRequest -Uri "http://localhost:8080/v1/sales/<UUID_VENTA>/pay" -Method POST `
  -Headers @{"Content-Type"="application/json";"X-Idempotency-Key"="test-key-001"} `
  -Body '{"method":"CARD","scenario":"APPROVED"}' | Select-Object -ExpandProperty Content

# Probar TIMEOUT → debe quedar UNKNOWN, nunca DECLINED
Invoke-WebRequest -Uri "http://localhost:8080/v1/sales/<UUID_VENTA>/pay" -Method POST `
  -Headers @{"Content-Type"="application/json";"X-Idempotency-Key"="test-key-002"} `
  -Body '{"method":"CARD","scenario":"TIMEOUT"}' | Select-Object -ExpandProperty Content

# Dashboard snapshot
Invoke-WebRequest -Uri "http://localhost:8080/v1/dashboard/today" | Select-Object -ExpandProperty Content
```

### Probar resiliencia del cache

```bash
# Apagar Dragonfly
docker compose stop dragonfly

# El dashboard sigue funcionando — reconstruye desde Postgres
curl http://localhost:8080/v1/dashboard/today

# Restaurar
docker compose start dragonfly
```

---

## Estructura del proyecto

```
movank/
├── backend/
│   ├── cmd/server/main.go          # Entry point
│   ├── internal/
│   │   ├── domain/                 # Modelos y tipos de dominio
│   │   ├── repository/             # Solo queries SQL — sin lógica de negocio
│   │   ├── service/                # Lógica de negocio (validaciones, outbox, idempotencia)
│   │   ├── handler/                # Parseo HTTP — solo llama al service
│   │   ├── worker/                 # Outbox worker (goroutine) + Hub SSE
│   │   ├── cache/                  # Abstracción sobre Redis/Dragonfly
│   │   └── db/                     # Pool de conexiones Postgres
│   ├── migrations/
│   │   └── 001_init.sql            # Tablas + trigger LISTEN/NOTIFY
│   └── Dockerfile
├── frontend/
│   ├── src/
│   │   ├── lib/
│   │   │   ├── db.ts               # IndexedDB — catálogo, carrito, ventas pendientes
│   │   │   ├── api.ts              # Comunicación con el backend
│   │   │   └── sync.worker.ts      # Web Worker — sincroniza PENDING_SYNC offline
│   │   └── routes/
│   │       ├── +page.svelte        # Catálogo
│   │       ├── cart/               # Carrito y pago
│   │       ├── receipt/            # Comprobante
│   │       └── dashboard/          # Dashboard con SSE
│   └── vite.config.ts              # Configuración PWA
├── docker-compose.yml
├── .env.example
└── README.md
```

---

## Decisiones técnicas

### Por qué Go en el backend


Se elijio go en el backend dado que es el lenguaje que se pedia principalmente, adicionalmente Go tiene simplifica el deploy y el razonamiento sobre concurrencia.

### Por qué el patrón outbox

Se realizo el uso de outbox dado que si el proceso muere entre el commit y el publish, la venta queda guardada pero el dashboard nunca se actualiza. Con outbox, el evento vive en Postgres en la misma transacción. Si el commit falla, el evento también se revierte. El worker lo procesa después, garantizando que ningún evento se pierde.

### Por qué TIMEOUT → UNKNOWN y no DECLINED

`DECLINED` significa que el proveedor rechazó el cobro lo que haria que se tiene una respuesta clara del provedor y se puede identificar. `TIMEOUT` significa que no sabemos qué pasó,puede que el cobro pudo haberse procesado o no. Tratarlo como `DECLINED` seria dar una respuesta al usuario sin tener clara la respuesta del proveedor. `UNKNOWN` queda para ser ejecutada nuevamente luego.

### Por qué idempotencia con `X-Idempotency-Key`

Se eligio de esta manera dado que si el cliente genera el key antes del intento, se podria tener el mismo key si la red falla y reintenta posteriormente, en estos casos al volver a enviar el key el backend detecta que ya procesó ese key y devuelve el resultado original sin volver a cobrar. por ello debe ser llave unica para que no se pueda generar una misma transacción dos veces.

### Por qué Redis/Dragonfly para el dashboard

Se hace el uso de dragonfly para poder disminuir la cantidad de peticiones a la base de datos. El cache tiene TTL de 30s como fallback, pero el outbox worker lo actualiza en cada venta aprobada lo cual hace que siempre esta actualizado.

### Por qué IndexedDB y no localStorage

`localStorage` es síncrono (bloquea el hilo principal), tiene límite de 5MB, y no soporta índices. IndexedDB es asíncrono, sin límite práctico de almacenamiento, y permite hacer queries por índice.

### Por qué el Web Worker para la sincronización

La sincronización de ventas pendientes implica múltiples peticiones HTTP. Si corre en el hilo principal, bloquea la UI mientras reintenta. El Web Worker corre en un hilo separado, se comunica con la UI por mensajes, y si muere a mitad de un intento, las ventas siguen en `PENDING_SYNC` en IndexedDB para el próximo ciclo.




