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
- ✅ Config-driven endpoints (JSON/YAML)
- ✅ Hot reload — save file, routes update instantly
- ✅ Status code simulation — 200, 404, 500, etc.
- ✅ Delay simulation — test latency
- ✅ Tiny footprint — single Go binary
- ✅ Docker support

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

## 📊 Roadmap

### v0.1 (MVP ✅)
- Config-driven endpoints
- Hot reload
- Status codes & delay
- Docker support

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
