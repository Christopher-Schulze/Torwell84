# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]
- Initial architecture outline in `TargetPicture.md`.
- Added technical analysis and optimization ideas.
- Documented hardware and network optimizations.
- Provided roadmap for phased implementation.
- Implemented Cloudflare Worker manager with `/workers` CRUD API and tests.
- Added connection state handling with `/connect`, `/disconnect`, `/status`,
  and log endpoints.
- Created initial VisionOS style UI skeleton with progress bar, node chain,
  country dropdowns, and Logs/Settings modals.
- Documented UI overview in README.
- Added worker health checking and duplicate detection.
- Backend now monitors local IP changes and logs them.
- Added round-robin worker selection with direct exit fallback when none are active.
- Worker endpoints persist to `workers.json` and reload on startup.
- New `/torrc` endpoint validates and stores custom configuration.
- Added `/config` endpoint to query and update OBFS4 and pre-warming settings persisted in `config.json`.
- Connection and system logs now persist to rotating files under the config directory.
- Log writer now flushes asynchronously to reduce I/O blocking.
- `/connect` accepts JSON with `entry`, `middle`, `exit` and `cflist` fields.
- Added `CircuitManager` with pre-warming of three circuits and rotation via `/connect` and `/new-circuit`.
- Implemented in-memory DNS cache and automatic BBR(v2) enable on Linux.
- Tor engine launches a tor process on connect and stops it on disconnect.
- Status endpoint now returns connection progress; UI polls to update the progress bar.
- Progress bar colour now reflects connection phase and worker health checks use the DNS cache.
- Connection state now persisted in state.json and restored on startup.
- Added graceful shutdown handling for server and background workers.
- New /metrics endpoint exposes basic Prometheus-style metrics.
- Tor engine reads `bridges.txt` when OBFS4 is enabled and passes bridges to tor.
- `/status` includes a human-readable status message.
- Added basic PWA support via `manifest.json` and `service-worker.js` in the UI.
- Consolidated `TechnicalAnalysis.md` into `TargetPicture.md`.
- Added experimental Tauri Mobile configuration and updated README with mobile build steps.
- Added `/workers/test` endpoint to verify Cloudflare Worker URLs and test all configured workers.
