# Torwell84

Torwell84 is a cross-platform Tor-VPN client powered by a Go backend and a Tauri + SvelteKit UI. It targets Windows, macOS, Linux, Android, iOS and Web with a VisionOS-inspired interface. The documentation lives in `TargetPicture.md` and `TechnicalAnalysis.md`.

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
DNS lookups are cached in memory for a short time and the server attempts to
enable BBR(v2) congestion control on Linux. Connection and system logs are
written asynchronously to rotating files under `logs/` inside this config
directory.

### API Quick Reference

The backend exposes a REST API on `127.0.0.1:9472` for controlling the Tor
client and managing Cloudflare Workers:

```text
GET  /status
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
```

### UI Overview

The SvelteKit/Tauri frontend displays a VisionOS style layout:

- A progress bar at the top shows connection state from 0–100%.
- A chain of five nodes (You, Entry, Middle, Exit, Cloudflare Worker) displays the
  current circuit. Entry, Middle and Exit have country dropdowns based on the
  static list in `TargetPicture.md`.
- Below the chain a button row allows Connect/Disconnect, New Circuit/Identity,
  and opens Logs and Settings modals.
- The Logs modal lists connection and system logs with Clear and Close buttons.
- Settings allow uploading a custom `torrc` verified through the `/torrc` endpoint (`tor --verify-config`), toggling OBFS4 and circuit
  pre-warming, and managing Cloudflare Worker endpoints. Workers can be added
  by entering the URL and hitting **Add**, and removed with the **Remove**
  button next to each entry. OBFS4 and pre-warming checkboxes immediately
  persist their state via the `/config` endpoint.

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

To compile both backend and UI in one step, run `./build.sh`. The script builds
all backend binaries and packages the SvelteKit/Tauri frontend into `ui/build/`.
