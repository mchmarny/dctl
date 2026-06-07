# devpulse

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

> **This project is archived.** A fully hosted, more powerful version of `devpulse` is available for **free** at **[devpulse.thingz.io](https://devpulse.thingz.io/)**.

## Use the hosted service

[**devpulse.thingz.io**](https://devpulse.thingz.io/) is the recommended way to use DevPulse. It's free, requires no install, and includes everything in this CLI plus features the standalone tool never had:

- **Zero setup** — sign in with GitHub, pick an org or repo, get insights
- **AI-powered insights** — narrative summaries and anomaly detection on top of the same metrics
- **Higher data limits** — no local GitHub API rate-limit juggling, no token pool to manage
- **Always up to date** — continuous imports, no `cron` or `sync` config to maintain
- **No database to manage** — no SQLite file, no disk usage, no migrations
- **Shareable dashboards** — point teammates at a URL instead of asking them to install a CLI

If you previously used the `devpulse` CLI, the hosted service covers every metric it produced (bus factor, lead time, change failure rate, contributor reputation, entity affiliations, release cadence, and the rest) with a richer dashboard.

**→ [Get started at devpulse.thingz.io](https://devpulse.thingz.io/)**

## About this repository

`devpulse` was a Go CLI that imported GitHub contribution data into a local SQLite database and served a browser-based analytics dashboard for community health metrics. Background on the original motivation is in [this blog post](https://blog.chmarny.com/posts/devpulse-community-health-analytics-for-the-rest-of-us/).

This repository remains available for reference and to honor existing installs, but is **no longer maintained**:

- No new features
- No bug fixes
- No security patches
- No new releases

The last release is still on the [releases page](https://github.com/mchmarny/devpulse/releases/latest) and the Homebrew tap will continue to serve it, but please migrate to the hosted service.

## License

[Apache 2.0](LICENSE)
