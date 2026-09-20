# Petstore Reference Architecture (`petstore-reference`)

A modern, production-grade reference microservice modeled after the classic Petstore domain, built with **[Go 1.27](https://go.dev)**, **[ConnectRPC](https://connectrpc.com)**, **[OpenTelemetry](https://opentelemetry.io)**, **[Buf](https://buf.build)**, **[protovalidate](https://github.com/bufbuild/protovalidate)**, **[FauxRPC](https://github.com/sudorandom/fauxrpc)**, **[sqlc](https://sqlc.dev)**, **[PostgreSQL](https://www.postgresql.org)**, and a **[React](https://react.dev)** + **[Vite](https://vite.dev)** frontend using **[TanStack Query](https://tanstack.com/query)** and **[Connect-Web](https://connectrpc.com/docs/web/getting-started)**.

This was put together [by request](https://github.com/sudorandom/kmcd.dev/issues/11). This shows how you can have static typing and validation for your APIs, your code (because Go) and database queries via SQLc.

---

## 🛠️ Tech Stack & Tooling

| Component | Tool / Library | Description |
| :--- | :--- | :--- |
| **Tooling Manager** | [mise](https://mise.jdx.dev) | Installs and manages `go`, `buf`, `sqlc`, `node`, `pnpm`, `fauxrpc`, etc. |
| **Language** | Go 1.27 | High-performance backend runtime |
| **RPC & API** | [ConnectRPC](https://connectrpc.com) | Multi-protocol RPC (Connect, gRPC, gRPC-Web) over HTTP/1.1 and HTTP/2 |
| **Observability** | [OpenTelemetry](https://opentelemetry.io) | Distributed tracing with W3C `traceparent` adoption via `otelconnect` and `otelpgx` |
| **Protobuf Management** | [Buf CLI](https://buf.build) | Linting, breaking change detection, and multi-language code generation |
| **Validation** | [protovalidate](https://buf.build/bufbuild/protovalidate) | Schema-level validation rules compiled into Protobuf definitions |
| **OpenAPI Generation** | [protoc-gen-connect-openapi](https://github.com/sudorandom/protoc-gen-connect-openapi) | Generates OpenAPI 3.1 specifications directly from Connect Protobuf definitions |
| **Testing & Mocking** | [FauxRPC](https://github.com/sudorandom/fauxrpc) | Fake Connect/gRPC/REST server with CEL dynamic stubs and failure simulation |
| **Integration Testing** | [Testcontainers for Go](https://golang.testcontainers.org) | Ephemeral PostgreSQL containers with automated Goose migrations and TRUNCATE isolation |
| **Database & ORM** | [sqlc](https://sqlc.dev) + [pgx/v5](https://github.com/jackc/pgx/v5) | Compile-time type-safe Go code generated from raw SQL queries |
| **Local Database** | Docker Compose | Local PostgreSQL container with automated schema migrations via Goose |
| **Linter & Security** | [golangci-lint](https://golangci-lint.run) + [gosec](https://github.com/securego/gosec) | Static analysis and security vulnerability scanner |
| **Vulnerability Scanner** | [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) | Official Go vulnerability scanner for known CVEs |
| **Frontend** | [React](https://react.dev) + [Vite](https://vite.dev) + [TanStack Query](https://tanstack.com/query) | Responsive SPA with Connect-Web, black/white dark mode toggle, and photo uploads |

---

## 📂 Project Structure

```
├── .mise.toml              # Toolchain versions (Go 1.27, Buf, sqlc, node, pnpm, fauxrpc)
├── .golangci.yml           # Linter configuration with gosec enabled
├── justfile                # Task runner commands (just)
├── docker-compose.yml      # Local PostgreSQL service
├── buf.yaml                # Buf module definition with protovalidate dependency
├── buf.gen.yaml            # Buf code generation for Go, OpenAPI, and TypeScript
├── sqlc.yaml               # SQLC configuration with pgx/v5 engine
├── cmd/
│   ├── migrate/            # Database migration CLI tool
│   └── server/             # Microservice entry point (CORS, h2c, interceptors, OTel, OpenAPI)
├── internal/
│   ├── auth/               # ConnectRPC Bearer Token authentication interceptor & context claims
│   ├── config/             # Environment variable configuration
│   ├── db/                 # SQLC generated database code & pgxpool with otelpgx
│   ├── pet/                # PetServiceHandler implementation & photo streaming handler
│   ├── telemetry/          # OpenTelemetry TracerProvider & Connect interceptor setup
│   └── testutil/           # PostgreSQL Testcontainers helper with Goose migrations & TRUNCATE
├── proto/
│   └── pet/v1/pet.proto    # Protobuf schema with validation rules
├── gen/                    # Generated Go stubs, OpenAPI specs, and binary descriptor images
├── sql/
│   ├── schema/             # Versioned Goose migrations
│   └── queries/            # SQLC queries for pets and photos
├── stubs/
│   ├── normal/             # FauxRPC stubs with CEL dynamic responses
│   └── failures/           # FauxRPC failure stubs for error testing
├── test/
│   └── integration_test.go # End-to-end ConnectRPC & PostgreSQL integration tests
└── web/                    # React + Vite frontend with TanStack Query and Connect-Web
    ├── src/
    │   ├── components/     # Layout, ThemeSwitcher, etc.
    │   ├── lib/            # Connect client, date utilities
    │   ├── pages/          # PetList, PetDetails, CreatePet, EditPet, Docs
    │   └── test/           # Vitest tests with ephemeral FauxRPC server
    └── package.json
```
---

## 🚀 Getting Started
 
### 1. Install Tooling with `mise`
```bash
just setup
```

### 2. Generate Code
Regenerate Protobuf, Connect stubs, OpenAPI specs, SQLC database code, and TypeScript types:
```bash
just generate
```

### 3. Start Local PostgreSQL with Docker Compose
```bash
just up
```

### 4. Code Quality, Security & Tests
```bash
# Run linter with gosec
just lint

# Run Go vulnerability check
just vulncheck

# Run internal tests (unit tests + database integration tests via Testcontainers)
just test

# Run end-to-end integration tests (ConnectRPC HTTP server + Testcontainers)
just test-integration

# Run frontend tests (Vitest + ephemeral FauxRPC mock server)
just test-web

# Run all quality & security checks at once (lint, vulncheck, test, test-web)
just check
```

### 5. Run the Go Microservice
```bash
just run
```
The service will be listening on `https://localhost:8080` (TLS enabled via `mkcert`).
- **Interactive OpenAPI Documentation:** `https://localhost:8080/docs`
- **OpenAPI 3.1 Spec (YAML):** `https://localhost:8080/openapi.yaml`
- **Health Check:** `https://localhost:8080/healthz`
- **Connect Service:** `https://localhost:8080/pet.v1.PetService/`

### 6. Run the Web Frontend
```bash
just web-dev
```
Open `https://localhost:4321` in your browser (TLS enabled via `mkcert`).
- **Web Interface:** `https://localhost:4321/`
- **Embedded API Documentation (Scalar):** `https://localhost:4321/docs`
- **OpenAPI 3.1 Spec (YAML):** `https://localhost:4321/openapi.yaml`

### 7. Run FauxRPC Standalone Mock Server
To run a mock server with fake data without starting PostgreSQL:
```bash
# Run with normal dynamic stubs (celfakeit)
just fauxrpc

# Run with failure stubs (simulating errors across all RPC methods)
just fauxrpc-fail
```
- **Mock Documentation:** `https://127.0.0.1:8080/fauxrpc/docs/`
FauxRPC will be available over HTTPS at `https://127.0.0.1:8080` with built-in documentation at `/fauxrpc/docs/`.

### 8. Testing the Frontend with FauxRPC
You can test the frontend against FauxRPC both interactively in the browser and via automated tests:

#### Interactive Development / Manual Testing
1. In one terminal, start the FauxRPC mock server (listening on `:8080`, the same port as the Go server):
   ```bash
   just fauxrpc
   # or test failure states:
   just fauxrpc-fail
   ```
2. In another terminal, start the web dev server:
   ```bash
   just web-dev
   ```
   The frontend at `https://localhost:4321` proxies all requests to `https://localhost:8080` (FauxRPC).

#### Automated Frontend Tests
Run the Vitest test suite, which automatically spawns ephemeral FauxRPC mock servers and verifies frontend pages, components, and error states:
```bash
just test-web
```
Or from the `web` directory:
```bash
pnpm test
```

---

## 🧪 Testing

Tests that touch the database run against real PostgreSQL (`postgres:17-alpine`) using **[Testcontainers for Go](https://golang.testcontainers.org)** instead of mocks or SQLite. This even includes top-level handler code, which makes those tests very powerful because they will interact with all layers beneath it without any mocking code. Very often, unit tests end up testing mock assertions more than your actual code if you leverage interfaces and mocking too often.

- **Automated migrations**: Containers start with the full suite of Goose migrations applied via [`db.Migrate`](internal/db/migrate.go).
- **Fast isolation via `TRUNCATE`**: To keep test suites fast (<3s), suites reuse the container and run `TRUNCATE TABLE pet_photos, pets RESTART IDENTITY CASCADE;` between tests instead of recreating containers.
- **Docker & Colima**: Automatically detects Colima on macOS (`~/.colima/default/docker.sock`). Set `DATABASE_URL` to point tests at an existing database instead.
- **Pure unit tests**: Logic without database dependencies (config, CORS, auth headers, validation helpers) runs in-memory.
- **Frontend mocks**: Web tests in `web/` use [FauxRPC](https://github.com/sudorandom/fauxrpc) stubs to test UI states without a running backend.

---

## 🔐 Authentication

In production, user login is handled by an upstream reverse proxy (like Google Cloud IAP or OAuth2 Proxy), which forwards user identity via headers. The service also supports static bearer tokens for machine-to-machine calls.

- **Proxy headers**: Reads identity from IAP (`X-Goog-Authenticated-User-*`) or OAuth2 Proxy (`X-Forwarded-*`) when `TRUST_PROXY_HEADERS=true`. Only enable this behind a proxy that strips untrusted client headers.
- **Bearer tokens**: Set `AUTH_TOKENS=token1,token2` for service-to-service or CLI access (`Authorization: Bearer <token>`).
- **Local dev**: When `DEV_MODE=true` (default), requests without credentials automatically use a dummy developer identity (`developer@local.test`).
- **Context**: Parsed claims are accessible in Go handlers via [`auth.FromContext(ctx)`](internal/auth/auth.go).
