# crosspile

A delbysoft TUI for browsing AI-agent request history across work locations.

`crosspile` scans configured work directories and local agent stores, then presents user prompts, assistant responses, sessions, projects, agents, models, and timestamps in one searchable interface.

## Config

The config file is created on first run at the platform config location:

- Linux: `~/.config/delbysoft/crossfile.toml`
- macOS: `~/Library/Application Support/delbysoft/crossfile.toml`
- Windows: `%APPDATA%\delbysoft\crossfile.toml`

On first launch, enter one or more work locations separated by commas.

## Filters

Press `/` to filter. Free text searches titles, session IDs, prompts, responses, projects, agents, and models.

Structured filters:

- `agent:opencode`
- `project:crosspile`
- `sid:ses_...`
- `model:gpt-5.5`
- `from:2026-05-01`
- `to:2026-05-17`
- `q:prompt text`
- `a:response text`

## Build

```bash
make build
```
