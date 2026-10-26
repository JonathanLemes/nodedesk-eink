# Contributing

Thanks for helping! NodeDesk E-Ink is small on purpose. Before a big change, open an issue to talk about it.

## Ground rules

- **Calm screen.** Few elements, large type, readable from across the room. Use the grays in `internal/render/draw.go` only (multiples of 17), never colours.
- **Smart server, dumb Kindle.** The client stays a busybox shell script with no dependencies beyond what jailbreaks ship (`curl`, FBInk, `lipc`). Anything heavier belongs in the server.
- **No hard-coded text.** Add strings to every language in `internal/render/i18n.go`.
- Keep changes focused and files small; match the surrounding style.

## Workflow

1. Fork and branch (`feat/…`, `fix/…`).
2. `make build`, then `./bin/nodedesk-eink render --demo -o out.png` to see your change without a Kindle.
3. `make test` must pass, and `shellcheck -s sh kindle/**/*.sh` for client changes.
4. Commit with [Conventional Commits](https://www.conventionalcommits.org/) (`feat: …`, `fix: …`, `docs: …`).
5. Open a pull request describing the *why*, with a rendered frame for layout changes (`make demo` updates the README images).

## Good first contributions

Translations, a new card (services, backups, weather), more GPU vendors, and reports from Kindle models other than the Paperwhite 5.
