# PartyShow

A company enters its website and a few insider facts and gets a finished, hosted quiz game show for its party. The show runs on a projector, and employees play on their phones by scanning a QR code. "PartyShow" is a placeholder name.

## What works today (no accounts needed)

| Part | Status |
|---|---|
| Landing page (German and English), demo button | ✅ |
| Create form: company, website, insider facts, teams, occasion, language, safe mode | ✅ |
| Question generation with OpenAI (`OPENAI_API_KEY`), or a built-in demo generator without a key | ✅ |
| Reads the company website (only public addresses, so it can't reach the server's internal network) | ✅ |
| Safe mode: filters questions about weight, age, salary, alcohol, religion, politics and more | ✅ |
| Editor: change, delete, reorder and add questions, set timers, add rounds | ✅ |
| Big screen: QR code and join code, rounds, timer, answer distribution, leaderboard, winners' podium, host voice (browser speech), keyboard control | ✅ |
| Phones: join without an app, answer, points for speed, rank, rejoin after reload or dropped connection | ✅ |
| Player limit per plan (free: 15) | ✅ |
| Rate limits on AI generation (per IP per hour, total per day), so nobody can burn the AI budget | ✅ |
| Automatic deletion of old shows (default 60 days) | ✅ |
| Tests including 200 simultaneous phones, plus a browser end-to-end test | ✅ |

## Still missing before launch

- **Payments (Stripe):** a checkout that unlocks the 50/200/500-player plans. Needs the owner's Stripe account.
- **Legal texts:** `web/impressum.html` and `web/datenschutz.html` contain placeholders and a draft. They need the owner's details and a checked text.
- **Final product name and domain.**
- **Host voice upgrade (optional):** better-sounding voices through an AI speech API. Today the show uses the voices built into the browser, which are free and work offline.
- **Affiliate tips:** gift and team-event recommendations on the final screen and in a follow-up email. These need labelled affiliate links.

## Run locally

```bash
go run ./cmd/server          # http://localhost:8080, demo generator
OPENAI_API_KEY=sk-... go run ./cmd/server   # with AI generation
go test ./...                # all tests (go test -short ./... skips the 200-phone load test)
```

Settings are environment variables, listed at the top of `cmd/server/main.go`.

## Deploy (one Hetzner Cloud server)

1. Create a server (Ubuntu, CX22 is enough to start) and install Docker.
2. Point the domain's A record at the server. With Cloudflare in front, set SSL/TLS mode to "Full".
3. Copy this folder to the server and create `deploy/.env`:
   ```
   BASE_URL=https://example.com
   OPENAI_API_KEY=sk-...
   TRUST_PROXY=true
   GENERATIONS_PER_DAY=200
   ```
4. Start it with `cd deploy && DOMAIN=example.com docker compose up -d --build`. Caddy gets the HTTPS certificate automatically.

Shows are stored as JSON files in the `data` volume. Running games live only in memory, so a restart ends running games but not saved shows.

## Code layout

```
cmd/server        entry point, configuration
internal/show     data model, validation, safe-mode filter
internal/generator  AI prompt + parsing (openai.go), demo generator (stub.go), clean-up (Finalize)
internal/webtext  website download and text extraction
internal/game     live game: rooms, players, timer, scoring, views for screen and phones
internal/store    shows as JSON files, edit tokens
internal/server   HTTP API, WebSockets, rate limits, QR codes
web/              pages, CSS and JavaScript (no build step, embedded into the binary)
```
