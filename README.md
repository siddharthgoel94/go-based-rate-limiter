# go-based-rate-limiter

A production-grade **API Gateway** built in Go featuring per-client rate limiting, JWT authentication, structured request logging, and PostgreSQL-backed analytics. Designed to demonstrate core backend engineering concepts including middleware chaining, Redis-based sliding window rate limiting, and async persistence patterns.

---

## Features

- **Rate Limiting** — Per-client sliding window algorithm using Redis, with configurable request limits and window sizes per client tier
- **JWT Authentication** — Stateless Bearer token validation with claims-based context injection
- **Request Logging** — Structured logging of every request (method, path, status, latency, client ID) with async PostgreSQL persistence via goroutines
- **REST API** — Clean route separation between public and protected endpoints
- **Zero Latency Overhead** — Database writes are fire-and-forget via goroutines, keeping the request path fast

---

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.21+ |
| Rate Limit Store | Redis (Upstash) |
| Database | PostgreSQL (Neon) |
| Auth | JWT (golang-jwt/jwt v5) |
| Config | godotenv |

---

## Architecture

```
Client Request
      │
      ▼
┌─────────────────┐
│  Logger         │  ← Records method, path, latency, status
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Rate Limiter   │  ← Sliding window via Redis · returns 429 if exceeded
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  JWT Auth       │  ← Validates Bearer token · returns 401 if invalid
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Router         │  ← /health · /token · /api/data · /api/logs
└────────┬────────┘
         │
    ┌────┴────┐
    ▼         ▼
 Redis     PostgreSQL
(limits)   (logs + analytics)
```

---

## Rate Limiting: Sliding Window Algorithm

Each client is identified by an `X-API-Key` header (falls back to IP). For every request:

1. Remove all timestamps from the Redis sorted set older than the window duration
2. Count remaining entries
3. If count < limit → allow and record the current timestamp
4. If count ≥ limit → reject with `429 Too Many Requests`

All four Redis operations are executed in a single **pipeline** (one round trip), making this efficient at high throughput.

```
Redis key: ratelimit:<client_id>
Structure: Sorted Set { member: timestamp_ms, score: timestamp_ms }
```

Response headers on every request:
```
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 7
Retry-After: 60   (only on 429)
```

---

## Project Structure

```
api-gateway/
├── cmd/
│   └── main.go                 # Entry point, wires middleware chain
├── internal/
│   ├── auth/
│   │   └── jwt.go              # Token generation + validation middleware
│   ├── middleware/
│   │   ├── ratelimiter.go      # Sliding window rate limiter
│   │   └── logger.go           # Request logger + async DB writer
│   ├── db/
│   │   └── postgres.go         # Connection, schema init, log persistence
│   └── handlers/
│       └── routes.go           # Route definitions and handlers
├── .env.example
├── go.mod
└── go.sum
```

---

## Getting Started

### Prerequisites

- Go 1.21+
- A free [Upstash](https://upstash.com) Redis instance
- A free [Neon](https://neon.tech) PostgreSQL instance

### 1. Clone the repository

```bash
git clone https://github.com/siddharthgoel94/go-based-rate-limiter.git
cd go-based-rate-limiter
```

### 2. Set up environment variables

```bash
cp .env.example .env
```

Edit `.env` with your credentials:

```env
JWT_SECRET=your-strong-random-secret-min-32-chars

# From Upstash → Database → Details tab
REDIS_ADDR=your-endpoint.upstash.io:6379
REDIS_PASSWORD=your-upstash-password

# From Neon → Dashboard → Connection string
DATABASE_URL=postgresql://user:password@ep-xxx.neon.tech/neondb?sslmode=require

PORT=8080
```

### 3. Install dependencies

```bash
go mod tidy
```

### 4. Run the server

```bash
go run cmd/main.go
```

Expected output:
```
PostgreSQL connected successfully
Database tables ready
API Gateway running on :8080
```

---

## API Reference

### Public Endpoints

#### `GET /health`
Health check. No authentication required.

```bash
curl http://localhost:8080/health
```
```json
{ "status": "ok" }
```

---

#### `GET /token?user_id=<id>`
Generate a JWT token for testing protected endpoints.

```bash
curl "http://localhost:8080/token?user_id=siddharth"
```
```json
{ "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." }
```

---

### Protected Endpoints

All protected endpoints require:
```
Authorization: Bearer <token>
X-API-Key: <your-client-id>
```

#### `GET /api/data`
Returns sample data for the authenticated user.

```bash
curl -H "Authorization: Bearer <token>" \
     -H "X-API-Key: siddharth" \
     http://localhost:8080/api/data
```
```json
{
  "message": "success",
  "user_id": "siddharth",
  "data": ["item1", "item2", "item3"]
}
```

---

#### `GET /api/logs`
Returns the last 20 request log entries from PostgreSQL.

```bash
curl -H "Authorization: Bearer <token>" \
     http://localhost:8080/api/logs
```
```json
[
  {
    "client_id": "siddharth",
    "path": "/api/data",
    "method": "GET",
    "status_code": 200,
    "latency_ms": 4,
    "created_at": "2026-04-19T08:00:00Z"
  }
]
```

---

## Testing Rate Limiting

Run this in your terminal to fire 15 requests rapidly and observe the 429s kick in after the limit is hit:

```bash
for i in {1..15}; do
  curl -s -o /dev/null -w "Request $i: %{http_code}\n" \
  -H "Authorization: Bearer <token>" \
  -H "X-API-Key: siddharth" \
  http://localhost:8080/api/data
done
```

Expected output:
```
Request 1: 200
Request 2: 200
...
Request 10: 200
Request 11: 429
Request 12: 429
...
```

---

## Database Schema

Tables are created automatically on first run.

```sql
CREATE TABLE request_logs (
    id          SERIAL PRIMARY KEY,
    client_id   TEXT NOT NULL,
    path        TEXT NOT NULL,
    method      TEXT NOT NULL,
    status_code INT NOT NULL,
    latency_ms  INT NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_client_created ON request_logs(client_id, created_at);
```

---

## Roadmap

- [ ] Multiple rate limiting strategies (Token Bucket, Fixed Window)
- [ ] Per-client tier configuration (free / pro / enterprise) stored in PostgreSQL
- [ ] Admin API for client management (CRUD + analytics)
- [ ] Circuit breaker for upstream service proxying
- [ ] Unit and integration tests with `httptest` and mock Redis
- [ ] Graceful shutdown handling

---

## Key Design Decisions

**Why sliding window over fixed window?**
Fixed window allows burst abuse at the boundary — a client can send `limit` requests just before the window resets and `limit` again right after, effectively doubling the allowed rate. Sliding window prevents this by always looking at the last N seconds from now.

**Why pipeline Redis commands?**
The four Redis operations (remove stale, count, add, expire) execute in a single round trip via `TxPipeline`, reducing network overhead significantly at high request rates.

**Why async DB writes?**
Request logs are written to PostgreSQL inside a goroutine so the client never waits for the DB write to complete. This keeps the gateway's p99 latency clean even if Postgres is temporarily slow.

---

## License

MIT