# Changelog

All notable changes to this project are documented in this file.

## Unreleased

### Added
- Minea scraper: optional **headless-free login** via AWS Cognito `InitiateAuth` (`USER_PASSWORD_AUTH`) using `MINEA_EMAIL` / `MINEA_PASSWORD` (same public app client id as the web app); falls back to Rod if Cognito rejects the flow (e.g. SRP-only, MFA, or wrong password). Set `MINEA_SKIP_COGNITO=true` to force browser login only.
- `cmd/import-minea-har`: build `minea_session.json` from a Chrome DevTools HAR by extracting the Cognito JWT and AppSync URL from a successful GraphQL POST.
- New API endpoint `POST /api/minea/search` to run live Minea ad searches with runtime filters from the Vue app.
- New Vue page `/minea-search` with filter controls (media type, running days, CTAs, EU-only, pagination, sort, CPM, and extra options) wired to live Minea search.
- Minea search now supports additional filter keys from `creator3` HAR (`ad_publication_date`, `ad_is_active`, `ad_languages`, `ad_countries`) and exposes searchable selectors in the Vue Minea Search page.

### Fixed
- Minea GraphQL default URL is **AWS AppSync** (`*.appsync-api.eu-west-1.amazonaws.com/graphql`), not `app.minea.com/graphql` (that path returns `307` to register). AppSync expects **raw JWT** in `Authorization` (no `Bearer ` prefix), matching the browser.
- Minea GraphQL from Go: disable HTTP redirect following (Vercel returns `307` JSON to `/graphql` then an HTML page—following it caused `invalid character '<'`). Persist browser cookies after Rod login and send `Cookie` plus `Origin`/`Referer`/`User-Agent` with each GraphQL POST so the session matches the app.
- Minea: treat `307` + JSON `redirect` (e.g. to `/en/register/...`) like auth failure—clear `minea_session.json` and run Rod login once when `GetCredits` / `GetTrendingProducts` hits GraphQL (stale disk sessions self-heal).
- Minea Rod login: pass Chromium `--no-sandbox` so headless browser starts on Linux when the SUID sandbox is unavailable.
- Minea login: default `https://app.minea.com/en/login/quickview?from=%2F` and `https://app.minea.com/graphql` (www Framer URLs are not the app); React-safe credential fill; Login/submit click fallbacks; polled JWT extraction from storage and cookies; Rod timeouts; fast fill unless `MINEA_SLOW_LOGIN=true`.

### Changed
- `cmd/scrape-minea`: human-readable zap development logs, stderr banner explaining long silent phases, and progress logs during Rod login.
- Minea RPC fallback search now paginates 3 pages by default and supports `.env` overrides via `MINEA_PAGE` (start page) and `MINEA_PAGES` (number of pages to fetch).
- Minea scraper hardcoded runtime defaults (origin, user-agent, credit thresholds/session cap, and RPC filter knobs) are now configurable via `.env` keys (`MINEA_ORIGIN`, `MINEA_USER_AGENT`, `MINEA_*CREDITS*`, `MINEA_AD_*`, `MINEA_CPM_VALUE`, `MINEA_EXCLUDE_BAD_DATA`).
- Product records now persist Minea image URLs in the store with upsert-on-id behavior, and the frontend Products table renders thumbnail previews.

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
