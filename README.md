# Torwell84

Torwell84 is a cross-platform Tor-VPN client powered by a Go backend and a Tauri + SvelteKit UI. It targets Windows, macOS, Linux, Android, iOS and Web with a VisionOS-inspired interface. The documentation lives in `TargetPicture.md`.

See `Roadmap.md` for the planned development phases.

## Build Instructions

1. Install Go 1.22+ and Node.js.
2. Build backend binaries:
   ```sh
   (cd backend && make all)
   ```
3. Setup UI dependencies:
   ```sh
   (cd ui && npm install)
   ```
4. Run Tauri development server:
   ```sh
   (cd ui && npm run dev)
   ```

The backend automatically monitors network IP changes and logs them so
connections can be re-established. Configured Cloudflare Worker endpoints are
persisted to disk and health checked every 30 seconds. When connecting,
Torwell84 selects the next active Worker in a round-robin fashion and falls
back to a direct exit if none are reachable. Circuits are kept pre-warmed using
a `CircuitManager` so new connections establish quickly. OBFS4 and pre-warming
preferences are saved in `config.json` and served through `/config`.
DNS lookups are cached in memory for a short time; worker health checks reuse this cache to avoid repeated resolves. The server attempts to
enable BBR(v2) congestion control on Linux. Connection and system logs are
written asynchronously to rotating files under `logs/` inside this config
directory.
Connection state is persisted in `state.json` and restored on startup. The
backend listens for `SIGINT`/`SIGTERM` to shut down the server and background
goroutines gracefully.
The `/connect` endpoint starts an embedded Tor process using any saved
`torrc` configuration, while `/disconnect` stops it.

### API Quick Reference

The backend exposes a REST API on `127.0.0.1:9472` for controlling the Tor
client and managing Cloudflare Workers:

```text
GET  /status  # returns {connected, progress, workers, config}
POST /connect       {"entry":"DE","middle":"FR","exit":"US","cflist":["https://w.example"]}
POST /disconnect
POST /new-circuit
POST /new-identity
POST /torrc (multipart file "file")
GET  /config
POST /config       {"obfs4":true,"prewarm":true}
GET  /logs/connection?level=debug
GET  /logs/general
GET  /workers
POST /workers    {"URL":"https://example.workers.dev"}
DELETE /workers  {"URL":"https://example.workers.dev"}
POST /workers/test   {"URL":"https://example.workers.dev"} # empty body tests all
GET  /metrics     # Prometheus style metrics
```

### UI Overview

The SvelteKit/Tauri frontend displays a VisionOS style layout:

- A progress bar at the top shows connection state from 0–100%, polling `/status` once per second. The bar starts purple while connecting, turns blue during handshake and becomes green once the circuit is ready.
- A chain of five nodes (You, Entry, Middle, Exit, Cloudflare Worker) displays the
  current circuit. Entry, Middle and Exit have country dropdowns based on the
  static list in `TargetPicture.md`.
- Below the chain a button row allows Connect/Disconnect, New Circuit/Identity,
  and opens Logs and Settings modals.
- The Logs modal lists connection and system logs with Clear and Close buttons.
- Settings allow uploading a custom `torrc` verified through the `/torrc` endpoint (`tor --verify-config`), toggling OBFS4 and circuit
  pre-warming, and managing Cloudflare Worker endpoints. Workers can be added
  by entering the URL and hitting **Add**. Each row has **Test** and **Remove**
  buttons, and a **Test All** button checks every configured Worker. OBFS4 and
  pre-warming checkboxes immediately persist their state via the `/config`
  endpoint.
- When OBFS4 is enabled, tor reads bridges from `bridges.txt` inside the config directory.

### Cross Compilation

The backend Makefile builds optimized binaries for major platforms. Example for
Apple M‑series with NEON acceleration:

```sh
GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build -tags neon -o torwell84-darwin-arm64
```

AVX2 builds for x86_64 follow the same pattern using `-tags avx2`.

Other targets can be built by setting the appropriate `GOOS` and `GOARCH` or
using the provided Makefile:

```sh
# Linux and Windows examples
GOOS=linux   GOARCH=amd64 CGO_ENABLED=1 go build -tags avx2 -o torwell84-linux-amd64
GOOS=linux   GOARCH=arm64 CGO_ENABLED=1 go build -tags neon -o torwell84-linux-arm64
GOOS=windows GOARCH=amd64 go build -o torwell84-windows-amd64.exe

# Mobile targets (requires the mobile toolchain)
GOOS=android GOARCH=arm64 go build -o torwell84-android-arm64
GOOS=darwin  GOARCH=arm64 GOIOS=1 go build -o torwell84-ios-arm64

# Or simply run
make -C backend all
```

### Mobile Builds
Tauri 2 provides experimental mobile support. After installing `@tauri-apps/cli` you can build and run the app on Android or iOS:

```sh
cd ui
npx tauri mobile android build   # Android APK
npx tauri mobile ios build       # iOS app
```

To compile both backend and UI in one step, run `./build.sh`. The script builds
all backend binaries and packages the SvelteKit/Tauri frontend into `ui/build/`.

### PWA
To build the web target with offline support, `manifest.json` and `service-worker.js` are provided under `ui/static`. Modern browsers will automatically install the Service Worker after running `npm run build` inside `ui/` and serving the `build/` directory.

### UI Tests

End-to-end tests are implemented with Playwright. Install the browsers and run the suite via:

```sh
cd ui && npm install && npm test
```

