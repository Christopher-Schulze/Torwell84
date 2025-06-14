# Implementation Plan

This file lists the remaining tasks to complete **Torwell84**. Follow the steps sequentially.

## 1. Backend Enhancements
- [x] Integrate the Tor engine with real tor binary and obfs4 bridges.
- [x] Persist connection state and worker list across restarts.
- [x] Implement automatic circuit rotation on `/new-circuit`.
- [x] Add graceful shutdown handling for all goroutines.
- [x] Expose metrics endpoint for debugging (optional).

## 2. Frontend Improvements
- [x] Finalize VisionOS style with GPU acceleration and animations.
- [x] Implement dropdowns for country selection as defined in `TargetPicture.md`.
- [x] Add connection progress color changes and detailed status messages.
- [x] Enable management of Cloudflare Worker URLs from the settings modal.
- [x] Add Playwright tests for main user flows.

## 3. Mobile and Web Targets
- [x] Configure Tauri Mobile for Android and iOS builds.
- [x] Add service worker and PWA manifest for the web target.

## 4. Performance and Hardware Optimizations
- [x] Build binaries with AVX2/NEON flags using the Makefile.
- [x] Enable BBRv2 automatically on Linux systems.
- [x] Introduce in-memory DNS and circuit caches.

## 5. Documentation and Cleanup
- [x] Update README with exact build commands for all platforms.
- [x] Keep CHANGELOG up to date for each feature.
- [x] Review and expand unit tests to cover new endpoints and functionality.
