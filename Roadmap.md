# Torwell84 Roadmap

This document outlines the recommended implementation phases for the project.

## Phase 1 – Skeleton
1. Create repository structure `/backend` and `/ui`.
2. Stub Go REST API with `/status` and `/connect` endpoints.
3. Basic Tauri + SvelteKit UI showing the node chain and buttons.
4. Integrate Tor via go-tor with default config.

## Phase 2 – Core Features
1. Implement circuit manager with pre-warming and OBFS4.
2. Add Cloudflare Worker management and failover logic.
3. Wire up connection progress bar and logs modal.
4. Enable custom `torrc` upload and validation.

## Phase 3 – Optimization
1. Compile binaries with AVX2/NEON flags.
2. Add in-memory caching for circuits and DNS responses.
3. Enable BBRv2 congestion control where available.
4. Implement GPU-accelerated transitions and 60 FPS rendering.

## Phase 4 – Mobile and Web
1. Setup Tauri Mobile alpha targets for Android and iOS.
2. Configure PWA build and service worker for web.
3. Validate performance goals on all platforms.

## Ongoing
- Maintain `CHANGELOG.md` for every feature.
- Update documentation when APIs or UI elements change.
