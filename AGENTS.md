# AGENTS.md

## Project overview
This repo contains **boylstonchessclub-tdbot**, a Go-based Tournament Director (TD) bot for the Boylston Chess Club.

It provides:
- A **Discord Interactions** HTTP service (`cmd/discordbot`) that implements `/td ...` slash commands.
- A **CLI** tool (`cmd/bcctd`) with similar commands for local use.
- A **cache seeding** utility (`cmd/cacheseed`) to warm USChess-related caches.
- A small S3-backed HTTP cache implementation (`s3cache`) used by the `uschess` client.

Primary data sources:
- Boylston Chess Club site / API (`bcc/*`)
- USChess MSA pages (`uschess/*`)

## Setup / build commands
This project is vendored.

- Build all binaries:
  - `make build`
- Run tests (uses vendored deps):
  - `make test`
- Update vendored deps (destructive; rewrites `go.mod`, `go.sum`, and `vendor/`):
  - `make deps`

Go toolchain:
- `go.mod` specifies Go **1.23** and toolchain **go1.23.10**.

## Repo layout
- `cmd/discordbot/` — Discord Interactions HTTP server (listens on `:8080`, handler at `/DiscordBot/Interaction`).
  - Embeds `token.priv`, `key.pub`, `app.id` at build time.
- `cmd/bcctd/` — CLI tool.
- `cmd/cacheseed/` — cache warmer.
- `bcc/` — Boylston Chess Club event/tournament scraping and formatting.
- `uschess/` — USChess client, parsing, formatting.
- `internal/httpcache/` — HTTP client with optional S3-backed caching.
- `s3cache/` — `httpcache.Cache` implementation backed by Amazon S3.
- `openapi/discord.json` — minimal OpenAPI for the DiscordBot service.

## Code style / conventions
- Language: Go.
- Prefer standard library patterns (contexts, errors, `net/http`).
- Keep bot responses under Discord message limits (see `truncateContent`).
- When changing output formatting, ensure Discord rendering stays readable (code blocks vs embeds).

## Testing guidance
Run:
- `make test`

Notes:
- Some tests behave like integration tests and may hit the network (USChess/BCC endpoints).
- S3-backed cache tests (`s3cache/*`, `internal/httpcache/*`) will **skip** if AWS credentials/bucket access are unavailable.

When modifying parsing/scraping logic:
- Add or update tests to cover representative HTML input/edge cases.
- Avoid introducing flakiness: prefer deterministic fixtures where possible.

## External services, credentials, and safety
### Discord credentials (build-time embedded)
`cmd/discordbot/main.go` embeds:
- `cmd/discordbot/token.priv` (bot token)
- `cmd/discordbot/key.pub` (public key used to verify incoming interactions)
- `cmd/discordbot/app.id` (Discord application id)

Agent rules:
- **Do not log, print, or exfiltrate secret contents.**
- **Do not replace or regenerate tokens/keys** unless explicitly requested.
- Treat `token.priv` as sensitive even if present in the working tree.

### AWS / S3 cache
S3 cache bucket name is currently hard-coded in `internal/constants.go` (`WebCacheBucket`).

- The cache uses AWS SDK default credential chain (env vars, shared config files, instance role, etc.).
- If cache init fails, HTTP clients may fall back to `http.DefaultClient` (no caching).

Agent rules:
- Don’t change bucket names/regions/permissions assumptions without an explicit request.
- Avoid adding new required infrastructure for tests.

## Common workflows
### Build and run the Discord bot service
- `make build`
- Run the produced `./discordbot` binary.
  - It listens on `:8080`.
  - Discord should be configured to send interaction POSTs to `/DiscordBot/Interaction`.

### Slash command registration
`cmd/discordbot/main.go` includes registration logic and a `lastupdate.hash` mechanism.
If you modify the command schema, the code may instruct you to update `cmd/discordbot/lastupdate.hash`.

Agent rule:
- Only update `lastupdate.hash` when you intentionally changed the command registration schema.

### Cache seeding
`cmd/cacheseed` will make many network requests and deliberately sleeps between calls.
Be mindful of load/ToS when modifying it.

## What to do when unsure
- Prefer reading relevant Go files and tests before refactoring.
- If a change might affect external behavior (Discord command signatures, output format, cache behavior), propose the change and call out compatibility concerns before implementing.
