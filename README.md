# Petstore Reference Architecture (`petstore-reference`)

A modern, production-grade reference microservice modeled after the classic Petstore domain, built with **Go 1.27**, **ConnectRPC**, **OpenTelemetry**, **Buf**, **protovalidate**, **FauxRPC**, **sqlc**, **PostgreSQL**, and a **React + Vite** frontend using **TanStack Query** and **Connect-Web**.

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
| **Integration Testing** | [Testcontainers for Go](https://golang.testcontainers.org) | Ephemeral PostgreSQL containers with automated schema initialization |
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
│   └── validator/          # protovalidate unary interceptor
├── proto/
│   └── pet/v1/pet.proto    # Protobuf schema with validation rules
├── gen/                    # Generated Go stubs, OpenAPI specs, and binary descriptor images
├── sql/
│   ├── schema/001_pets.sql # DDL schema for PostgreSQL (consolidated)
│   └── queries/            # SQLC queries for pets and photos
├── stubs/
│   ├── normal/             # FauxRPC stubs with CEL dynamic responses
│   └── failures/           # FauxRPC failure stubs for error testing
├── test/
│   └── integration_test.go # PostgreSQL & OpenTelemetry integration tests
└── web/                    # React + Vite frontend with TanStack Query and Connect-Web
    ├── src/
    │   ├── components/     # Layout, ThemeSwitcher, etc.
    │   ├── lib/            # Connect client, date utilities
    │   ├── pages/          # PetList, PetDetails, CreatePet, EditPet, Docs
    │   └── test/           # Vitest tests with ephemeral FauxRPC server
    └── package.json
```

---

## 🔐 Authentication Architecture

Authentication assumes an upstream reverse proxy or ingress auth layer (e.g. **Google Cloud IAP**, **OAuth2 Proxy**, **Envoy OAuth**, **Cloudflare Access**) that terminates user login at the boundary:
- **Upstream Identity Extraction**:
  - **Google Cloud IAP**: Automatically extracts user email (`X-Goog-Authenticated-User-Email`), subject ID (`X-Goog-Authenticated-User-Id`), and optional JWT assertions (`X-Goog-IAP-JWT-Assertion`).
  - **OAuth2 Proxy / Ingress**: Automatically extracts email (`X-Forwarded-Email`), user (`X-Forwarded-User`), and groups/roles (`X-Forwarded-Groups`).
  - **Service-to-Service Fallback**: Also accepts standard `Authorization: Bearer <token>` for machine-to-machine or CLI API calls.
- **Context Propagation**: Authenticated caller [`Claims`](internal/auth/auth.go) are injected into `context.Context` (accessible via `auth.FromContext(ctx)`).
- **Seamless Local Development**: In development mode (`DEV_MODE=true`), requests without upstream proxy headers automatically receive a default local dev identity, so the frontend and backend work out of the box without manual token inputs.
- **Web Frontend**: The browser Connect-ES client uses `credentials: "include"` so session cookies are automatically passed to the proxy. No manual tokens are needed in the UI.

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

# Run unit tests
just test

# Run all checks at once (lint, vulncheck, unit tests)
just check

# Run integration tests (PostgreSQL & FauxRPC)
just test-integration
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
Open `https://localhost:5173` in your browser (TLS enabled via `mkcert`).
- **Web Interface:** `https://localhost:5173/`
- **Embedded API Documentation (Scalar):** `https://localhost:5173/docs`
- **OpenAPI 3.1 Spec (YAML):** `https://localhost:5173/openapi.yaml`

### 7. Run FauxRPC Standalone Mock Server
To run a mock server with fake data without starting PostgreSQL:
```bash
# Run with normal dynamic stubs (celfakeit)
just fauxrpc

# Run with failure stubs (simulating errors across all RPC methods)
just fauxrpc-fail
```
- **Mock Documentation:** `https://127.0.0.1:6660/fauxrpc/docs/`
FauxRPC will be available over HTTPS at `https://127.0.0.1:6660` with built-in documentation at `/fauxrpc/docs/`.

### 8. Testing the Frontend with FauxRPC
You can test the frontend against FauxRPC both interactively in the browser and via automated tests:

#### Interactive Development / Manual Testing
1. In one terminal, start the FauxRPC mock server:
   ```bash
   just fauxrpc
   # or test failure states:
   just fauxrpc-fail
   ```
2. In another terminal, start the web dev server configured for mock mode:
   ```bash
   just web-mock
   ```
   The frontend at `https://localhost:5173` will proxy all Connect-RPC requests to FauxRPC (`https://127.0.0.1:6660`) instead of the real backend.

#### Automated Frontend Tests
Run the Vitest test suite, which automatically spawns ephemeral FauxRPC mock servers and verifies frontend pages, components, and error states:
```bash
just test-web
```
Or from the `web` directory:
```bash
pnpm test
```

