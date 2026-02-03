## mocknest

mocknest is a lightweight WireMock-style mock server written in Go.  
You define HTTP mocks as JSON files, and the server matches incoming requests to the **best matching mock** and returns the configured response.

The goal is to be:
- **Deterministic**: clear, predictable matching rules
- **File-based**: all mocks live as JSON on disk
- **Introspectable**: admin endpoints expose loaded mocks and call history

---

## 1. Requirements

- **Go** 1.21+ installed (`go version` should work)
- `curl` (for quick manual testing)

---

## 2. Getting Started

Clone the repo and enter the project:

```bash
git clone <your-fork-or-origin-url> mocknest
cd mocknest
```

Install dependencies (Go modules):

```bash
go mod tidy
```

### 2.1. Running the Server (Production Mode)

Run the server directly:

```bash
go run ./server
```

By default the server listens on port **8342**:

- Base URL: `http://localhost:8342`

You can override the port:

```bash
PORT=8080 go run ./server
```

### 2.2. Development with Air (Hot Reload)

For development, use **[Air](https://github.com/cosmtrek/air)** to automatically rebuild and restart the server when you change Go files.

#### Installing Air

Install the latest version of Air:

```bash
# Using Go install (recommended)
go install github.com/cosmtrek/air@latest

# Verify installation
air -v
```

Alternatively, you can install via other package managers:

```bash
# macOS (Homebrew)
brew install air

# Linux (using install script)
curl -sSf https://air-installer.netlify.app/install.sh | sh -s -- -b $(go env GOPATH)/bin
```

#### Using Air

The project includes a `.air.toml` configuration file. Simply run:

```bash
air
```

Air will:
- Watch all `.go` files in the `server/` directory
- Automatically rebuild when you save changes
- Restart the server with zero downtime
- Show build errors in the terminal

**Note**: Air watches Go source files by default. Changes to mock JSON files in `mocks/` do **not** trigger rebuilds (the server reloads mocks on each startup).

To customize Air's behavior, edit `.air.toml` in the project root.

#### Air Configuration

The included `.air.toml` is configured to:
- Build the server binary to `./tmp/main`
- Watch `.go` files (excluding `_test.go` files)
- Ignore `tmp/`, `vendor/`, and `testdata/` directories
- Use colored output for better readability

You can override the port when using Air:

```bash
PORT=8080 air
```

---

## 3. Mock JSON format

Mocks live under the `mocks/` directory (e.g. `mocks/test-1.json`, `mocks/test-2.json`).  
On startup, the server loads all `*.json` files from that directory.

### 3.1. Example mock

```json
{
  "id": "create-user-order",
  "description": "Mock response for user order creation",
  "request": {
    "method": "POST",
    "urlPattern": "/users_orders",
    "queryParams": {
      "userId": "123"
    },
    "body": {
      "orderType": "ALL"
    }
  },
  "response": {
    "status": 201,
    "headers": {
      "Content-Type": "application/json"
    },
    "body": {
      "orderId": "MOCK-ORDER-001",
      "status": "CREATED",
      "message": "Order created successfully"
    },
    "fixedDelayMs": 0
  },
  "metadata": {
    "tags": [
      "orders",
      "users",
      "integration-test"
    ],
    "enabled": true
  }
}
```

### 3.2. Fields

- **`id`**: Unique identifier for the mock. Used in admin views and call history.
- **`description`**: Human-readable description.

- **`request`**:
  - **`method`**: HTTP method to match (e.g. `"GET"`, `"POST"`, `"PUT"`). Case-insensitive.
  - **`urlPattern`**: Path to match, e.g. `"/users_orders"`.  
    - For now this is a simple string; matching is done against the request path.
  - **`queryParams`**: Object of **key → value** pairs that must match exactly in the incoming request.
    - Example: `"queryParams": { "userId": "123", "source": "mobile" }`
    - All listed key–value pairs must be present for the mock to match.
  - **`body`**: Object of **JSON field-path → expected value**.
    - Paths use simple dot notation (no arrays yet), e.g.:
      - `"orderType": "ALL"`
      - `"customer.email": "test@example.com"`
    - All listed fields must exist and equal the configured values.

- **`response`**:
  - **`status`**: HTTP status code to return (e.g. `200`, `201`, `403`).
  - **`headers`**: Object of header name → value. `"Content-Type": "application/json"` is added if missing.
  - **`body`**: Any JSON-serializable payload to return as the response body.
  - **`fixedDelayMs`**: Optional artificial delay in milliseconds before sending the response (simulates latency).

- **`metadata`**:
  - **`tags`**: Arbitrary labels for grouping/search (used only by admin/introspection, not matching).
  - **`enabled`**: If `false`, the mock is **ignored** at load time.

---

## 4. Matching behavior

At runtime, all mocks are loaded into an in-memory index.  
For each incoming request:

- **Step 1** – Normalize the request:
  - Method uppercased (e.g. `"post"` → `"POST"`)
  - URL path: `r.URL.Path`
  - Query: `r.URL.Query()` (map of `string → []string`)
  - Body: parsed as JSON if possible, otherwise raw string

- **Step 2** – Filter candidates by:
  - HTTP method
  - URL path pattern

- **Step 3** – Check detailed constraints:
  - All configured `queryParams` (key + exact value) must match.
  - All configured `body` fields must exist and equal the configured value.

- **Step 4** – Choose the **best** match:
  - First by **priority** (lower `priority` wins; default is `1000` if omitted).
  - Then by a **specificity score**:
    - More constraints (query + body) → higher score.
  - Then by load order (stable tie-break).

If no mapping matches, the server returns:

```json
{
  "error": "no mock mapping found",
  "method": "<HTTP_METHOD>",
  "url": "/requested/path"
}
```

with status `404`.

---

## 5. Admin endpoints

Admin endpoints are exposed under the `"/__admin"` namespace:

- **`GET /__admin/mocks`**
  - Returns all loaded mock mappings (the parsed `Mapping` structs).
  - Useful to verify that your JSON files were loaded and interpreted correctly.

  Example:

  ```bash
  curl -s http://localhost:8342/__admin/mocks | jq .
  ```

- **`GET /__admin/history`**
  - Returns an in-memory list of all calls the mock server has processed since startup.
  - Each record (a `CallRecord`) contains:
    - `time`: timestamp
    - `method`: HTTP method
    - `url`: path
    - `query`: query map
    - `requestBody`: parsed request body (if JSON)
    - `mappingId`: ID of the matched mock (empty if no mock matched)
    - `status`: HTTP status returned

  Example:

  ```bash
  curl -s http://localhost:8342/__admin/history | jq .
  ```

> **Note**: history is **not persisted**. It is kept only in memory and cleared on process restart.

---

## 6. Example usage

### 6.1. Happy-path mock

Given the example mock above, you can trigger it with:

```bash
curl -i \
  -X POST "http://localhost:8342/users_orders?userId=123" \
  -H "Content-Type: application/json" \
  -d '{"orderType":"ALL"}'
```

You should see:

- Status: `201`
- JSON body with the configured `orderId`, `status`, and `message`.

### 6.2. Unauthorized (403) mock

You can define another mock (e.g. `mocks/test-2.json`) like:

```json
{
  "id": "users-orders-unauthorized",
  "description": "Return 403 when user is not authorised to create orders",
  "request": {
    "method": "POST",
    "urlPattern": "/users_orders",
    "queryParams": {
      "userId": "999"
    },
    "body": {
      "orderType": "ALL"
    }
  },
  "response": {
    "status": 403,
    "headers": {
      "Content-Type": "application/json"
    },
    "body": {
      "error": "unauthorized",
      "message": "User is not allowed to create orders",
      "userId": "999"
    },
    "fixedDelayMs": 0
  },
  "metadata": {
    "tags": [
      "orders",
      "users",
      "auth"
    ],
    "enabled": true
  }
}
```

Trigger it with:

```bash
curl -i \
  -X POST "http://localhost:8342/users_orders?userId=999" \
  -H "Content-Type: application/json" \
  -d '{"orderType":"ALL"}'
```

You should get a `403` with the configured error payload.

---

## 7. Running tests

From the project root:

```bash
go test ./server/...
```

This runs unit tests, including the core matching logic in `server/appdata`.

---

## 8. Containerization with Docker

mocknest includes a `Dockerfile` for building and running the server in a containerized environment.

### 8.1. Building the Docker Image

Build the Docker image:

```bash
docker build -t mocknest:latest .
```

This creates a multi-stage build:
- **Builder stage**: Uses `golang:1.25-alpine` to compile the Go binary
- **Final stage**: Uses minimal `alpine:latest` image with only the compiled binary

**Note**: The current Dockerfile does not copy the `mocks/` directory into the image. To include mocks in the image, you can either:
- Mount the `mocks/` directory as a volume (see section 8.2)
- Modify the Dockerfile to `COPY mocks/ /app/mocks/` before the final stage

### 8.2. Running the Container

Run the container:

```bash
docker run -p 8342:8342 mocknest:latest
```

The server will be available at `http://localhost:8342`.

#### Custom Port Mapping

To map to a different host port:

```bash
docker run -p 8080:8342 mocknest:latest
```

Then access the server at `http://localhost:8080`.

#### Mounting Mock Files

If you want to update mocks without rebuilding the image, mount the `mocks/` directory:

```bash
docker run -p 8342:8342 \
  -v $(pwd)/mocks:/app/mocks \
  mocknest:latest
```

**Note**: The server loads mocks at startup. To reload mocks after changes, restart the container:

```bash
docker restart <container-id>
```

### 8.3. Docker Compose (Optional)

Create a `docker-compose.yml` for easier management:

```yaml
version: '3.8'

services:
  mocknest:
    build: .
    ports:
      - "8342:8342"
    volumes:
      - ./mocks:/app/mocks
    environment:
      - PORT=8342
    restart: unless-stopped
```

Run with:

```bash
docker-compose up -d
```

Stop with:

```bash
docker-compose down
```

### 8.4. Production Considerations

- **Air is for development only**: Do not use Air in production Docker containers. The Dockerfile builds a static binary and runs it directly.
- **Mock persistence**: Mocks are loaded from the `mocks/` directory at startup. For production, either:
  - Bake mocks into the image (COPY mocks/ into the image)
  - Mount a volume with your mock files
  - Use a config management system
- **Health checks**: Consider adding a health check endpoint (e.g., `GET /health`) for container orchestration.

---

## 9. Future ideas

Some directions you can expand mocknest:

- **Richer matching**:
  - Header matchers
  - Body JSONPath / array support
  - Operators like `equals`, `contains`, `regex`, `oneOf`
- **Stateful mocks**:
  - Scenario support (different responses over time)
- **Web UI**:
  - Visual mock editor
  - Live call history view and search

For now, the project is intentionally small and strict so that the behavior is easy to understand and reason about.

