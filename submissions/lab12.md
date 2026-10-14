# Lab 12 — WebAssembly Containers

## Environment

- Machine: Apple MacBook Pro, M4 Pro (Apple Silicon)
- OS: macOS, ARM64
- Go: 1.27.1
- Spin: 4.2.1
- Spin Go SDK: v3.0.0
- TinyGo: 0.43.0
- Wasmtime: 49.0.3
- Benchmark tool: hyperfine

## Task 1 — HTTP Endpoint with Spin

### Implementation

The application was scaffolded using the official Spin Go HTTP template.

Source files:

- [main.go](../wasm/quicknotes-time/main.go)
- [spin.toml](../wasm/quicknotes-time/spin.toml)
- [go.mod](../wasm/quicknotes-time/go.mod)
- [go.sum](../wasm/quicknotes-time/go.sum)

The application registers an HTTP handler using `spinhttp.Handle`.

The `/time` endpoint returns Moscow time (UTC+3) as JSON, including:

- `unix`: Unix timestamp in seconds
- `iso`: RFC3339 timestamp
- `hour_minute`: time formatted as HH:MM

The response uses `Content-Type: application/json`.

Outbound network access is disabled using:

```toml
allowed_outbound_hosts = []
```

### Build

```bash
cd wasm/quicknotes-time
spin build
```

The official Spin 4.2.1 Go template uses:

```bash
go tool componentize-go build
```

This differs from the TinyGo-based build command described for Spin 3.x in the assignment. The current template was preserved to maintain compatibility with Spin 4.2.1 and the Spin Go SDK v3.

Build result:

```text
main.wasm: 5,682,680 bytes
```

### Verification

```bash
spin up
curl -i http://127.0.0.1:3000/time
```

Observed response:

```http
HTTP/1.1 200 OK
content-type: application/json

{"unix":1792006287,"iso":"2026-10-14T22:31:27+03:00","hour_minute":"22:31"}
```

The endpoint returned HTTP 200, valid JSON, and the correct Moscow UTC offset.

### Design Questions

**a) Browser WASM vs server WASM**

Browser WebAssembly typically uses JavaScript bindings and browser APIs to interact with the page and access browser-provided functionality.

Server-side WASI modules do not have direct access to the DOM or browser JavaScript APIs. Instead, they use host-provided interfaces for capabilities such as standard input/output, clocks, and permitted filesystem operations.

WASI makes WebAssembly suitable for server-side and command-line workloads without requiring a browser.

**b) Why is `-buildmode=c-shared` needed?**

In the TinyGo-based Spin SDK v2 workflow described in the assignment, `-buildmode=c-shared` helps produce a module with the expected exports and ABI for the Spin host.

Without the required exports, the host cannot invoke the HTTP handler correctly.

However, Spin 4.2.1 uses the newer component model. Its official Go template builds a WASI HTTP component using `componentize-go`, so the legacy TinyGo build flag is not applicable to our generated project.

**c) Capability-based security vs Docker `--network none`**

WebAssembly components receive access to host resources through explicitly granted capabilities.

In this application, `allowed_outbound_hosts = []` prevents the component from making outbound HTTP requests through Spin's permitted networking interfaces.

Docker's `--network none` isolates the container from external networking at the container network namespace level.

Both restrict networking, but WASM uses a host-mediated capability model rather than relying primarily on operating-system namespaces.

**d) TinyGo standard library limitations**

TinyGo does not provide full compatibility with every part of the standard Go runtime and standard library.

Common limitations include timezone database availability and some reflection-heavy operations.

During development, loading `Europe/Moscow` initially failed in the WebAssembly environment because timezone data was unavailable. We resolved this by embedding timezone data with `time/tzdata`.

The final Spin component uses the current Go component toolchain, while the standalone WASI bonus module is compiled with TinyGo.

---

## Task 2 — Docker vs WASM/Spin Performance

### Test Setup

Both applications were tested locally on the same MacBook Pro M4 Pro.

Endpoints:

- Docker: `http://127.0.0.1:8081/health`
- Spin: `http://127.0.0.1:3000/time`

Docker image: `quicknotes:lab6`

For warm latency, each endpoint was benchmarked using 5 warmup runs and 50 measured runs.

```bash
hyperfine --warmup 5 --runs 50 \
  'curl -fsS -o /dev/null http://127.0.0.1:8081/health'
```

```bash
hyperfine --warmup 5 --runs 50 \
  'curl -fsS -o /dev/null http://127.0.0.1:3000/time'
```

For cold start, each runtime was stopped and restarted five times. The elapsed time was measured from the launch command until the first successful HTTP response.

### Performance Results

| Dimension | Lab 6 Docker | Lab 12 WASM/Spin |
|---|---:|---:|
| Artifact size | 21.5 MB | 5.68 MB |
| Cold start (p50) | 98.74 ms | 50.27 ms |
| Warm latency (p50) | 7.500 ms | 7.739 ms |
| Warm latency (p95) | 9.066 ms | 8.702 ms |

### Cold-Start Samples

| Run | Docker | Spin |
|---|---:|---:|
| 1 | 107.13 ms | 128.39 ms |
| 2 | 115.60 ms | 49.05 ms |
| 3 | 93.62 ms | 49.39 ms |
| 4 | 98.74 ms | 56.10 ms |
| 5 | 90.50 ms | 50.27 ms |

Spin achieved a lower median cold-start time, while warm latency was similar for both platforms.

The first Spin launch was noticeably slower than subsequent launches, potentially due to initialization or caching effects.

### Measurement Limitations

Warm latency was measured using the total execution time of the `curl` command, including process startup overhead.

The cold-start measurements included the launch command and the time until the first successful response.

Docker measurements used an existing local image and container, so image download and extraction were not included.

Spin and Docker expose different endpoints and use different runtime architectures. Therefore, the results represent a practical local comparison rather than an isolated measurement of application execution speed.

### Design Questions

**e) What dominates cold start?**

Docker startup can involve container creation, namespace and cgroup initialization, filesystem setup, and application process startup. Image extraction may also contribute when an image is not already available locally.

Spin must start its runtime, load or compile the WebAssembly component, instantiate it, and initialize the HTTP trigger.

In our measurements, the Docker image was already present, so downloading and extracting the image were not part of the measured startup time.

**f) When is WASM better, and when is Docker preferable?**

WASM is attractive for small, isolated services, serverless functions, plugins, and multi-tenant workloads where fast startup and restricted capabilities are useful.

Docker remains preferable for applications requiring full operating-system compatibility, native dependencies, existing Linux tooling, or complex multi-process services.

**g) Multi-tenant safety**

A WASM sandbox makes unauthorized access to host resources more difficult because the module can only use interfaces exposed by the host.

For example, a malicious tenant attempting to read another tenant's files or access unauthorized network destinations faces explicit capability restrictions.

Linux containers also provide isolation, but their security depends on operating-system mechanisms such as namespaces, cgroups, capabilities, and seccomp.

WASM is not immune to vulnerabilities, but its restricted host interface can reduce the available attack surface.

---

## Bonus Task — Standalone WASI with Wasmtime

### Implementation

The standalone application is implemented without the Spin SDK.

Source:

- [wasm-cli/main.go](../wasm-cli/main.go)

It reads `REQUEST_METHOD` and `PATH_INFO` from environment variables and writes an HTTP-style response to standard output.

The application returns the same Moscow-time JSON fields as the Spin endpoint.

### Build

```bash
cd wasm-cli
tinygo build -o main.wasm -target=wasi -no-debug ./main.go
```

Build result:

```text
main.wasm: 209,032 bytes
```

### Run

```bash
wasmtime run \
  --env REQUEST_METHOD=GET \
  --env PATH_INFO=/time \
  main.wasm
```

Observed output:

```http
Content-Type: application/json

{"unix":1792006670,"iso":"2026-10-14T22:37:50+03:00","hour_minute":"22:37"}
```

### Performance Comparison

The standalone module was benchmarked using 50 separate invocations of `wasmtime run`, with 5 warmup runs.

```bash
hyperfine -N --warmup 5 --runs 50 \
  'wasmtime run --env REQUEST_METHOD=GET --env PATH_INFO=/time main.wasm'
```

Results:

| Dimension | Spin HTTP Component | Standalone WASI CLI |
|---|---:|---:|
| Module size | 5,682,680 bytes | 209,032 bytes |
| Startup / invocation p50 | 50.27 ms | 4.370 ms |
| Execution model | Persistent HTTP server | New CLI invocation per request |

Standalone Wasmtime results:

- Runs: 50
- Mean: 4.603 ms
- Median (p50): 4.370 ms
- Minimum: 3.975 ms
- Maximum: 11.785 ms

The standalone module was much smaller and had a shorter measured invocation time.

However, this is not a direct HTTP performance comparison: Spin starts an HTTP server, while the standalone module only executes a CLI program.

### Design Questions

**h) Why cannot the Spin component run under bare `wasmtime run`?**

The Spin application is a WASI HTTP component that exposes an HTTP handler through the component model.

A standalone WASI CLI module instead provides a command-style entry point.

The two modules expect different host interfaces and execution models. The Spin component therefore requires a compatible WASI HTTP host, such as Spin or an appropriately configured `wasmtime serve`.

**i) What does Spin add on top of Wasmtime?**

Spin provides an application framework around the Wasmtime runtime, including HTTP routing, trigger management, application manifests, component lifecycle management, and capability configuration.

It also manages HTTP request handling and outbound-host restrictions, removing the need to implement these features manually.

**j) When should each execution model be used?**

Per-invocation `wasmtime run` is suitable for short command-line jobs, isolated scripts, and batch processing where each execution is independent.

Spin's persistent HTTP server is more suitable for APIs, webhooks, and lightweight web services that must continuously accept HTTP requests.

---

## Conclusion

The lab demonstrates two WebAssembly execution models and compares them with a traditional Docker container.

Spin provided a working HTTP endpoint with a smaller artifact and lower measured median cold-start time than the Lab 6 container.

Standalone Wasmtime produced an even smaller module and shorter per-invocation execution time, but it does not provide an HTTP server by itself.

The results illustrate the trade-off between lightweight isolated execution and a managed HTTP application runtime.
