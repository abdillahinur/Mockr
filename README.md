# Mockr

> Mock APIs in seconds — hot-reloading, zero dependencies, CI/CD ready.

Mockr is a lightweight mock server for developers.  
Unlike [JSON Server](https://github.com/typicode/json-server), [Mockoon](https://mockoon.com), or [WireMock](https://wiremock.org), Mockr is designed to be **fast, simple, and developer-friendly**.  

- ⚡ **Hot reload** your config file — no restarts needed.  
- 🐳 **Docker and CLI ready** — runs anywhere in seconds.  
- 🚦 **Simulate real-world conditions** with delays and custom status codes.  
- 🎯 **Zero overhead** — one binary or one Docker container.  

---

## ✨ Features (v0.1)
- ✅ **Config-driven endpoints** (JSON/YAML) with validation
- ✅ **Hot reload** — save file, routes update instantly with symlink safety
- ✅ **Status code simulation** — 200, 404, 500, etc. with validation
- ✅ **Delay simulation** — test latency with 30s safety cap
- ✅ **Rate limiting** — optional token bucket protection against abuse
- ✅ **Health checks** — `/health` endpoint for Docker/K8s readiness
- ✅ **Request logging** — privacy-safe observability (method, path, status, duration)
- ✅ **Security hardened** — localhost binding, timeouts, body limits, graceful shutdown
- ✅ **Tiny footprint** — single Go binary with zero dependencies
- ✅ **Docker support** — multi-stage build, non-root user, minimal image

---

## 🚀 Quickstart

### 1. Build locally
```bash
go build -o mockr ./cmd/mockr
./mockr start examples/mockr.json
```

### 2. Test an endpoint
```bash
curl localhost:3000/ping
# {"ok": true}
```

### 3. Run with Docker
```bash
docker build -t mockr .
docker run -p 3000:3000 -v $(pwd)/examples/mockr.json:/app/mockr.json mockr
```

## 🛠️ How to Use in Your Existing Project

### Integration Options

#### **Option 1: Development Dependencies**
Add Mockr as a dev dependency to your project for local development:

```bash
# Download and use directly
curl -L https://github.com/abdillahi-nur/mockr/releases/latest/download/mockr-linux-amd64 -o mockr
chmod +x mockr
./mockr start your-api-mocks.json
```

#### **Option 2: Docker Compose Integration**
Add to your `docker-compose.yml` for team development:

```yaml
version: '3.8'
services:
  mockr:
    build: .
    ports:
      - "3000:3000"
    volumes:
      - ./api-mocks.json:/app/mockr.json
    command: ["mockr", "start", "/app/mockr.json"]
  
  your-app:
    build: .
    environment:
      - API_BASE_URL=http://mockr:3000
    depends_on:
      - mockr
```

#### **Option 3: CI/CD Integration**
Use Mockr in your CI pipeline for testing:

```yaml
# GitHub Actions example
- name: Start Mock API
  run: |
    ./mockr start tests/api-mocks.json &
    sleep 2
    
- name: Run Tests
  run: |
    npm test -- --api-base=http://localhost:3000
    
- name: Stop Mock API
  run: pkill mockr
```

### Configuration Examples

#### **API Testing Scenarios**
```json
{
  "routes": {
    "/api/users": {
      "method": "GET",
      "status": 200,
      "response": [
        {"id": 1, "name": "Alice", "email": "alice@example.com"},
        {"id": 2, "name": "Bob", "email": "bob@example.com"}
      ]
    },
    "/api/users/1": {
      "method": "GET", 
      "status": 200,
      "delay": 500,
      "response": {"id": 1, "name": "Alice", "email": "alice@example.com"}
    },
    "/api/users": {
      "method": "POST",
      "status": 201,
      "response": {"id": 3, "message": "User created"}
    },
    "/api/error": {
      "method": "GET",
      "status": 500,
      "response": {"error": "Internal server error"}
    }
  }
}
```

#### **Load Testing Setup**
```bash
# Start with rate limiting for load testing
./mockr start --rate-limit=100 --burst=200 api-mocks.json

# Test with artillery, k6, or wrk
wrk -t12 -c400 -d30s http://localhost:3000/api/users
```

#### **Frontend Development**
```javascript
// Set API base URL in your app
const API_BASE_URL = process.env.NODE_ENV === 'development' 
  ? 'http://localhost:3000' 
  : 'https://api.production.com';

// Use in your API calls
fetch(`${API_BASE_URL}/api/users`)
  .then(response => response.json())
  .then(data => console.log(data));
```

#### **Backend Integration Testing**
```go
// Go test example
func TestUserAPI(t *testing.T) {
    // Start mockr server
    cmd := exec.Command("mockr", "start", "test-mocks.json")
    cmd.Start()
    defer cmd.Process.Kill()
    
    // Wait for server to start
    time.Sleep(2 * time.Second)
    
    // Test against mock API
    resp, err := http.Get("http://localhost:3000/api/users")
    assert.NoError(t, err)
    assert.Equal(t, 200, resp.StatusCode)
}
```

### Production Considerations

#### **Environment Variables**
```bash
# Development
export MOCK_API_URL=http://localhost:3000

# Staging (with rate limiting)
./mockr start --rate-limit=50 --burst=100 --host=0.0.0.0 staging-mocks.json

# Testing (with health checks)
./mockr start test-mocks.json
curl http://localhost:3000/health  # Verify readiness
```

#### **Monitoring & Logging**
```bash
# Enable request logging (always on)
./mockr start api-mocks.json

# Monitor logs
tail -f mockr.log | grep "GET\|POST\|PUT\|DELETE"

# Health check monitoring
watch -n 5 'curl -s http://localhost:3000/health'
```

#### **Security in Production**
```bash
# Bind to localhost only (default)
./mockr start api-mocks.json

# Use behind reverse proxy
nginx -c nginx.conf  # Proxy to localhost:3000

# Docker with security flags
docker run --read-only --tmpfs /tmp -p 3000:3000 mockr
```

## 🔧 Command Line Options

```bash
./mockr start [flags] <configFile>

Flags:
  -host string
        Host to bind to (default "127.0.0.1")
  -port int
        Port to run the server on (default 3000)
  -watch
        Enable hot reload file watching (default true)
  -rate-limit float
        Rate limit in requests per second (default 0 = disabled)
  -burst int
        Burst size for rate limiting (default 0; only used if rate-limit > 0)
```

### External Access
To allow external connections (not recommended for production), explicitly set host:
```bash
./mockr start --host 0.0.0.0 --port 3000 examples/mockr.json
```

## 🚦 Rate Limiting

Protect your mock server from abuse with optional rate limiting:

```bash
# Enable rate limiting: 5 requests/second with burst of 10
./mockr start --rate-limit=5 --burst=10 examples/mockr.json

# Test rate limiting
for i in {1..20}; do curl -s localhost:3000/ping & done
# Some requests will return 200, others 429 {"error":"rate_limited"}
```

**Rate limiting features:**
- Uses token bucket algorithm for smooth rate limiting
- Returns HTTP 429 with JSON error when limit exceeded
- Completely bypassed when `--rate-limit=0` (default)
- Applied to all routes except `/health` endpoint

## 🏥 Health Checks

Mockr includes a built-in health endpoint for container orchestration:

```bash
curl localhost:3000/health
# {"status":"ok"}
```

**Health endpoint features:**
- Always returns HTTP 200 with `{"status":"ok"}`
- Not affected by config delays or status overrides
- Perfect for Docker/Kubernetes liveness and readiness probes
- Lightweight and fast response

## 📊 Request Logging

All requests are logged with concise, privacy-safe information:

```
2024/01/15 10:30:45 GET /ping 200 12ms
2024/01/15 10:30:46 GET /api/users 200 5ms
2024/01/15 10:30:47 GET /ping 429 0ms
2024/01/15 10:30:48 GET /health 200 1ms
```

**Logging features:**
- Format: `method path status duration_ms`
- No request bodies or headers logged (privacy-safe)
- Includes all routes including `/health`
- Shows rate-limited requests with 429 status

## 📝 Example Config (examples/mockr.json)
```json
{
  "routes": {
    "/ping": {
      "method": "GET",
      "status": 200,
      "response": { "ok": true }
    },
    "/users": {
      "method": "GET",
      "delay": 1000,
      "response": [
        { "id": 1, "name": "Alice" },
        { "id": 2, "name": "Bob" }
      ]
    }
  }
}
```

- `method`: HTTP verb (GET, POST, PUT, DELETE, etc.)
- `status`: optional HTTP status code (defaults to 200)
- `delay`: optional artificial delay in ms
- `response`: the JSON body returned

## 🔒 Security

**Default Behavior:**
- Server binds to `127.0.0.1` (localhost only) by default
- No CORS headers enabled
- Request body size limited to 1MB
- Delay capped at 30 seconds maximum
- HTTP timeouts configured to prevent slowloris attacks
- Docker container runs as non-root user
- Rate limiting disabled by default

**External Access:**
To allow external connections, explicitly set host:
```bash
./mockr start --host 0.0.0.0 config.json
```

**Production Considerations:**
- Run behind a reverse proxy (nginx, traefik)
- Use HTTPS termination at proxy level
- Monitor request logs for abuse
- Consider running Docker container with `--read-only` flag
- Enable rate limiting for public endpoints

**Docker Security:**
```bash
# Enhanced security with read-only filesystem
docker run --read-only --tmpfs /tmp -p 3000:3000 -v $(pwd)/config.json:/app/mockr.json mockr
```

## 📊 Roadmap

### v0.1 (MVP ✅)
- ✅ Config-driven endpoints with validation
- ✅ Hot reload with symlink safety
- ✅ Status codes & delay with security bounds
- ✅ Optional rate limiting (token bucket)
- ✅ Health checks (`/health` endpoint)
- ✅ Privacy-safe request logging
- ✅ Security hardening (localhost bind, timeouts, body limits)
- ✅ Graceful shutdown with signal handling
- ✅ Docker support (multi-stage, non-root user)

### v1.0 (Coming soon 🚧)
- Chaos mode (random errors)
- Dynamic responses (params, queries, body injection)
- Faker data generation
- CLI flags & profiles

### v2.0 (Future 💡)
- OpenAPI export
- Record & replay real APIs
- CI/CD ephemeral mode
- Playground GUI (/__mockr)

## 🤝 Contributing
Contributions are welcome!  
Open an issue to suggest features, report bugs, or discuss improvements.

## 📜 License
MIT