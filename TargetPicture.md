# Project Torwell84

## Overview
Torwell84 is a cross-platform Tor-VPN client combining a Go backend with a Tauri + SvelteKit frontend. The goal is to deliver a single binary per platform with a modern VisionOS "Liquid Glass" user interface and full GPU acceleration via WGPU/Metal/Vulkan. Supported targets are Windows 10+, macOS 12+ (x86_64 and arm64/Neon), Linux (glibc 2.31+), Android 10+, iOS 15+, and the Web via PWA.

### Why Tauri + SvelteKit?
* **Small footprint** – Tauri bundles only a lightweight Rust core and a WebView, resulting in binaries around a few megabytes instead of full Electron size.
* **GPU acceleration** – WGPU allows hardware-accelerated rendering via Metal, Vulkan or DirectX, enabling the VisionOS "Liquid Glass" style at 60 FPS.
* **Official integration guides** – Tauri provides step‑by‑step docs for SvelteKit and ships mobile wrappers starting with v2, letting the same codebase target Android and iOS.
* **Web-first** – SvelteKit builds to a performant PWA for the web target without additional tooling.

## Architecture
### Backend (`torwell84/backend`)
- **Language:** Go 1.22+
- **Tor integration:** embed Tor using Go bindings such as `go-tor`. OBFS4 bridges are enabled by default. A custom `torrc` can be uploaded through the UI.
- **Cross compilation:** static binaries for Windows, Linux and macOS are produced via the Makefile. Mobile builds require the Go mobile toolchain.
  Example commands:
  ```sh
  # macOS Apple Silicon
  GOOS=darwin  GOARCH=arm64 CGO_ENABLED=1 go build -tags neon -o torwell84-darwin-arm64
  # Windows
  GOOS=windows GOARCH=amd64 go build -o torwell84-windows-amd64.exe
  # Linux AMD64
  GOOS=linux   GOARCH=amd64 CGO_ENABLED=1 go build -tags avx2 -o torwell84-linux-amd64
  # Android/iOS
  GOOS=android GOARCH=arm64 go build -o torwell84-android-arm64
  GOOS=darwin  GOARCH=arm64 GOIOS=1 go build -o torwell84-ios-arm64
  ```
  Neon intrinsics may be enabled via `// #cgo CFLAGS: -mfpu=neon` on ARM builds while AVX2 is selected with `-tags avx2` on x86_64.
- **REST API** served on `127.0.0.1:9472`:
  - `GET  /status`
  - `POST /connect` with JSON `{entry, middle, exit, cflist[]}`
  - `POST /disconnect`
  - `POST /new-circuit`
  - `POST /new-identity`
  - `POST /torrc` – upload custom configuration
  - `GET  /config` – retrieve current OBFS4 and pre-warming settings
  - `POST /config` – update settings `{obfs4, prewarm}`
  - `GET  /logs/connection?level=debug`
  - `GET  /logs/general`
  - `GET  /workers` – list configured Cloudflare Workers
  - `POST /workers` – add a new Worker `{URL}` (health checked before accepting)
  - `DELETE /workers` – remove Worker `{URL}`
- **Circuit manager**
  - Keeps three circuits pre-warmed at all times.
  - Stores the last healthy Cloudflare Worker endpoint and runs a health check every 30 s (requests `/.well-known/healthz`).
  - Uses round-robin selection across healthy Workers and falls back to direct Tor exit if none respond.
- **Logging**
  - Rotating JSON log files under `$APPDATA/torwell84/logs` (or platform equivalent).
  - Connection logs at DEBUG level, general logs at INFO level.

### Backend Modules
The backend is organised into loosely coupled modules so features remain easy to
maintain and extend:

| Module          | Purpose                                                        |
|-----------------|----------------------------------------------------------------|
| `torengine`     | Embeds the Tor daemon via Go bindings and exposes a control API |
| `api/http`      | Implements the REST endpoints listed above                      |
| `circuit-manager` | Handles pre‑warming, country selection and worker failover    |
| `proxy-cf`      | Optional HTTPS forwarder to the Cloudflare Worker endpoints    |
| `logging`       | Collects connection and general logs with rotation to files under the config directory |


### Frontend (`torwell84/ui`)
- **Framework:** Tauri 2 with SvelteKit and TypeScript.
- **Style:** VisionOS “Liquid Glass” aesthetic using CSS `backdrop-filter`, depth effects and GPU-accelerated animations at 60 FPS.
- **Layout:**
  1. **Top progress bar**
     - Tor-style bar from 0–100 % with color phases (connecting, handshaking, establishing circuit, ready).
     - Updates in real time during connection and disconnect.
  2. **Node chain** (five icons in a row)
     - **You**: fixed label "U" with local flag.
     - **Entry node**
     - **Middle node**
     - **Exit node**
     - **Cloudflare Worker** (greyed out when no Worker configured).
     - Each node shows Tor node name, country flag, and IP address beneath the icon. The three Tor nodes have a **country dropdown** above them for selecting the desired country.
  3. **Button row** beneath the chain
     - Connect / Disconnect (toggle)
     - New Circuit / New Identity (toggles based on state)
     - Logs (opens modal)
     - Settings (opens modal)
  4. **Logs modal**
     - Tabs: *Connection*, *System*, *All*.
     - Each tab lists logs with `Copy` and `Clear` buttons.
     - Logs are cleared on application start.
  5. **Settings modal**
     - Upload custom `torrc` file; validate with `tor --verify-config` before applying.
     - Toggle OBFS4 bridges (default **ON**).
     - Toggle circuit pre-warming (default **ON**).
    - Manage Cloudflare Worker list: add new endpoint, test it via `/.well-known/healthz`, remove existing ones. Working endpoints are persisted in `workers.json` and selected in round-robin order.
    - OBFS4 and pre-warming toggles are stored in `config.json` alongside the worker file.

#### Additional UI details
* **Progress bar colours**: purple while connecting, blue during handshake and green once the circuit is ready.
* **Icons**: a laptop for "You", onion symbols for the Tor nodes and the Cloudflare logo for the Worker.
* **Logs modal**: opens in a floating "glass" pane with tabs along the top. Each tab shows a scrollable text area and `Copy`/`Clear` buttons on the bottom right.
* **Settings modal**: contains checkboxes and toggles styled as iOS switches. The Cloudflare Worker table lists the URL and last health check result with `Add`, `Test` and `Remove` buttons.
* **Responsive layout**: on mobile the node chain stacks vertically and the button row becomes a column to remain finger-friendly.
- **Static country list** for dropdowns (in this order):
  `["Deutschland","Frankreich","Belgien","Schweiz","Liechtenstein","Luxemburg","Österreich","Spanien","Italien","Portugal","Russland","Rumänien","Türkei","UK","USA","Kanada","Mexiko","Brasilien","Argentinien","Japan","China","Antarktis"]`
- **Adaptive design**: UI scales for desktop and mobile; touch input supported on mobile.

### Cloudflare Worker Proxy
- Optional fourth hop after the Tor exit node. Traffic is forwarded through user-supplied HTTPS Cloudflare Worker endpoints.
- Health checks every 30s. The backend rotates through healthy Workers in round-robin order. If none respond, traffic is sent directly through the Tor exit node.

### Hardware and Network Optimization
- **CPU features:** binaries include AVX2 on x86_64 and NEON on ARM64 to accelerate cryptography and compression.
- **GPU acceleration:** WGPU uses Metal on macOS (including Apple M-series), Vulkan on Windows/Linux, and OpenGL ES on mobile devices to maintain smooth 60 FPS.
- **TCP congestion control:** the backend attempts to enable BBRv2 on Linux at startup to improve throughput on high latency links.
- **In-memory cache:** recent circuit descriptors and DNS responses are stored in memory via a `dnsCache` module to minimize latency.
- **IP monitoring:** the backend watches for local IP changes and logs them so circuits can be re-established seamlessly.
- **Encryption:** all proxy connections enforce TLS 1.3 with ChaCha20-Poly1305 preferred, falling back to AES-GCM.

### Mobile and Web
- Tauri Mobile alpha wrapper for Android and iOS using the same SvelteKit codebase.
- When built for Web, served as a PWA with a Service Worker for offline caching.

### Performance Targets
- Backend API latency < 100 ms under 1k concurrent connections.
- Frontend animations remain at 60 FPS with TTI < 50 ms on modern hardware.

## Deliverables
1. `/backend` – Go sources with a Makefile (`make all` cross-compiles the static binaries).
2. `/ui` – SvelteKit project including Tauri configuration for desktop, mobile and web.
3. `build.sh` – one-liner script that compiles the project for all targets and places outputs into platform folders.
4. `README.md` – instructions on building, running, and adding Cloudflare Worker endpoints.
5. `TargetPicture.md` – this architecture document.

## Build & Deployment
1. Install Go 1.22+ and Node.js.
2. Run `make -C backend all` to build backend binaries for desktop targets.
3. Inside `ui/`, execute `npm install` followed by `npm run build` to compile the SvelteKit frontend.
4. Execute `./build.sh` to produce release artifacts that combine backend and UI into platform folders.
5. For mobile builds, install the Go mobile toolchain and use the commands shown in the cross compilation section.

## Additional Notes
- Cloudflare Worker section is disabled (grey) until at least one Worker is configured.
- Logs start empty on each launch and are viewable through the Logs modal.
- Name “Torwell84” nods to a cyberpunk twist on *Orwell 1984*.
- Sample cross‑compile for macOS on Apple Silicon:
  ```sh
  GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build -tags "neon" -o torwell84-darwin-arm64
  ```
