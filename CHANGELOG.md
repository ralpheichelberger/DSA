# Changelog

All notable changes to this project are documented in this file.

## Unreleased

### Added
- Step 1: Project scaffold, Go module setup, Makefile targets, config loader, env template, and initial entrypoint.
- Step 2: SQLite store layer with migrations, models, CRUD operations, and memory context builder.
- Step 3: Product scoring and campaign decision engine with comprehensive tests.
- Step 3a: Expanded static knowledge base and system prompt assembly utilities.
- Step 4: OpenAI integration client with mocked HTTP tests and JSON parsing helpers.
- Step 5: Integration interfaces and realistic stubs for Minea, Sup, Meta, and TikTok.
- Step 5a: Multi-channel owner notification package (log, Slack, Discord, Telegram, Email) with timeout/error handling.
- Step 6: Real Minea GraphQL scraper with Rod login bootstrap, credit checks, session persistence, and parsing tests.
- Step 7: Real Sup, Meta, and TikTok HTTP clients behind existing interfaces with dev-mode stub fallback.
- Step 8: Main agent loop implementing DISCOVER -> REASON -> APPROVE -> LAUNCH -> MONITOR -> LEARN phases.
- Step 9: Chi HTTP API (health, products, campaigns, lessons, approve, chat) plus WebSocket hub broadcasting agent outbox; `httptest` coverage.
- Step 10: `cmd/agent` wires config, store, integrations, agent loop, and HTTP server with signal-based shutdown; `cmd/smoke-test` and Makefile `smoke-test` / `full-test`; `api.Server.Serve` for ephemeral listeners.
- Step 11: Vue 3 + Vite dashboard (Dashboard, Products, Campaigns, Chat), `useApi` / `useWebSocket` composables, Vite proxy to `localhost:8080`, indigo theme and system UI styling.
