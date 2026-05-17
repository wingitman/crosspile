# crosspile

A TUI for browsing AI-agent request history across work locations.

`crosspile` scans configured work directories and local agent stores, then presents user prompts, assistant responses, sessions, projects, agents, models, and timestamps in one searchable interface.

## Config

The config file is created on first run at the platform config location:

- Linux: `~/.config/delbysoft/crossfile.toml`
- macOS: `~/Library/Application Support/delbysoft/crossfile.toml`
- Windows: `%APPDATA%\delbysoft\crossfile.toml`

On first launch, enter one or more work locations separated by commas or new lines.

Examples:

- Linux/macOS: `~/Work, ~/Projects`
- Windows: `%USERPROFILE%\Work, D:\Projects`

You can also edit the config directly with one block per root:

```toml
[[locations]]
name = "Work"
path = "~/Work"

[[locations]]
name = "Projects"
path = "~/Projects"
```

Config edits are reloaded automatically while the TUI is running. If locations or enabled agents change, `crosspile` rescans.

## Updates

`crosspile` follows the same git-backed updater design as the other local tools:

- The installer stamps the binary with commit, origin, and repo path metadata.
- Install metadata is written to `crosspile-install-meta.toml` in the delbysoft config directory.
- Startup checks run in the background using noninteractive git commands.
- If an update is available, a popup shows recent commits and asks before pulling/reinstalling.
- On confirmation, a detached updater waits for the TUI to exit, then runs `git pull` and `make install` on Linux/macOS or `install.ps1` on Windows.

Updater behavior is configurable in `[updates]`, and the update-check key is configurable in `[keybinds]`.

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

Build all release targets:

```bash
make build-all
```

Windows:

```powershell
.\install.ps1
.\install.ps1 -BuildAll
```


## Support
<a href='https://ko-fi.com/W7W21WP5L7' target='_blank'><img height='36' style='border:0px;height:36px;' src='https://storage.ko-fi.com/cdn/kofi4.png?v=6' border='0' alt='Buy Me a Coffee at ko-fi.com' /></a>

## License

MIT — see [LICENSE](LICENSE).

Copyright (c) 2026 [delbysoft](https://github.com/wingitman)
