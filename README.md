# ytrack

`ytrack` is a small cross-platform command-line client for YouTrack. It keeps
shared credentials in a global config file, keeps project-specific settings in
the current repository, and gives you short commands for creating issues and
moving their status.

The project is an early MVP. It currently supports:

- global YouTrack URL and token configuration
- local per-directory URL, token, and project configuration
- creating YouTrack issues
- changing issue status through YouTrack commands
- raw JSON output for API-backed commands

## Installation

### Homebrew

```sh
brew install rtcoder/tap/ytrack
```

Development builds from the main branch are available with:

```sh
brew install --HEAD rtcoder/tap/ytrack
```

### GitHub Releases

Download a release package from:

```text
https://github.com/rtcoder/ytrack/releases
```

Debian/Ubuntu users can install the release `.deb` package:

```sh
sudo apt install ./ytrack_<version>_linux_amd64.deb
```

Fedora/RHEL users can install the release `.rpm` package:

```sh
sudo dnf install ./ytrack_<version>_linux_amd64.rpm
```

Release assets include:

- macOS Intel: `ytrack_<version>_darwin_amd64.tar.gz`
- macOS Apple silicon: `ytrack_<version>_darwin_arm64.tar.gz`
- Linux x64: `ytrack_<version>_linux_amd64.tar.gz`
- Linux ARM64: `ytrack_<version>_linux_arm64.tar.gz`
- Debian/Ubuntu x64: `ytrack_<version>_linux_amd64.deb`
- Debian/Ubuntu ARM64: `ytrack_<version>_linux_arm64.deb`
- Fedora/RHEL x64: `ytrack_<version>_linux_amd64.rpm`
- Fedora/RHEL ARM64: `ytrack_<version>_linux_arm64.rpm`
- Windows x64: `ytrack_<version>_windows_amd64.zip`
- SHA-256 hashes: `checksums.txt`

### Build From Source

```sh
go build -o ytrack ./cmd/ytrack
```

Or install into your Go bin directory:

```sh
go install ./cmd/ytrack
```

## Quick Start

Set your shared YouTrack URL and token once:

```sh
ytrack global set-url https://youtrack.example.com
ytrack global set-token perm:your-token
```

Inside a repository, set the YouTrack project ID:

```sh
ytrack set-project-id 0-1
```

Create an issue:

```sh
ytrack issue create "Crash on save" "Steps to reproduce..."
```

Move an issue to another status:

```sh
ytrack issue status ART-123 Done
```

## Configuration Model

Configuration is merged at runtime:

```text
local ./.ytrack/config.json > global OS user config
```

Local values override global values. This lets you keep one global token while
each repository chooses its own YouTrack project.

`project_id` is local-only by design. It is never read from or written to the
global config, which helps avoid creating issues in the wrong project.

### Config Files

Global config lives under Go's `os.UserConfigDir()`:

- macOS: usually `~/Library/Application Support/ytrack/config.json`
- Linux: usually `~/.config/ytrack/config.json`
- Windows: usually `%APPDATA%\ytrack\config.json`

Local config lives in the current working directory:

```text
./.ytrack/config.json
```

Config files containing tokens are written with `0600` permissions where the
operating system supports POSIX file modes.

Add this to repositories where you use local config:

```gitignore
.ytrack/
```

### Inspect Config

Show the effective merged config:

```sh
ytrack show
```

Show only global config:

```sh
ytrack global show
```

Tokens are masked in output:

```text
url: https://youtrack.example.com
token: perm:xxxx...abcd
project_id: 0-1
```

## Command Reference

### Global Options

```sh
ytrack --help
ytrack --json <command>
```

`--json` prints raw JSON for commands that call the YouTrack API. It is useful
for scripts and shell pipelines.

### Global Configuration

```sh
ytrack global show
ytrack global set-url <url>
ytrack global set-token <token>
ytrack global unset-url
ytrack global unset-token
```

Global config is best for values shared across projects, such as the YouTrack
base URL and a personal permanent token.

Examples:

```sh
ytrack global set-url https://youtrack.example.com
ytrack global set-token perm:your-token
ytrack global show
ytrack global unset-token
```

There is no `ytrack global set-project-id`. Project IDs are intentionally local.

### Local Project Configuration

```sh
ytrack show
ytrack set-url <url>
ytrack set-token <token>
ytrack set-project-id <project-id>
ytrack unset-url
ytrack unset-token
ytrack unset-project-id
```

Local config is best for repository-specific overrides. The most common local
setting is `project_id`.

Examples:

```sh
ytrack set-project-id 0-1
ytrack set-url https://team.youtrack.example.com
ytrack set-token perm:project-specific-token
ytrack show
ytrack unset-project-id
```

### Issues

```sh
ytrack issue create <summary>
ytrack issue create <summary> <description>
ytrack issue status <issue-id> <status>
```

Create an issue with only a summary:

```sh
ytrack issue create "Crash on save"
```

Create an issue with a summary and description:

```sh
ytrack issue create "Crash on save" "Steps to reproduce..."
```

Change issue status:

```sh
ytrack issue status ART-123 Done
ytrack issue status ART-123 "In Progress"
```

`issue status` sends a YouTrack command in this form:

```text
State <status>
```

### JSON Output

Use `--json` before the command:

```sh
ytrack --json issue create "Crash on save"
ytrack --json issue create "Crash on save" "Steps to reproduce..."
ytrack --json issue status ART-123 Done
```

Without `--json`, successful commands print human-readable output:

```text
Created ART-123: "Crash on save"
Updated ART-123 to Done
```

With `--json`, API-backed commands print the raw YouTrack response.

## Required Configuration

`issue create` requires:

- `url`
- `token`
- local `project_id`

`issue status` requires:

- `url`
- `token`

Missing values produce actionable errors, for example:

```text
missing configured project_id, run `ytrack set-project-id <project-id>`
```

## YouTrack Token

Create a permanent token in YouTrack and pass it to:

```sh
ytrack global set-token perm:your-token
```

If a repository needs different credentials, override the token locally:

```sh
ytrack set-token perm:project-specific-token
```

## Development

Run tests:

```sh
go test ./...
```

Build the CLI:

```sh
go build -o ytrack ./cmd/ytrack
```

Cross-compile examples:

```sh
GOOS=darwin GOARCH=arm64 go build -o dist/ytrack-darwin-arm64 ./cmd/ytrack
GOOS=linux GOARCH=amd64 go build -o dist/ytrack-linux-amd64 ./cmd/ytrack
GOOS=windows GOARCH=amd64 go build -o dist/ytrack-windows-amd64.exe ./cmd/ytrack
```

## Releases

GitHub Actions publishes release packages when a version tag is pushed:

```sh
git tag v0.1.1
git push origin v0.1.1
```

You can also run the `Release` workflow manually from GitHub Actions and provide
an existing tag.

The release workflow:

1. runs tests
2. builds Linux, macOS, and Windows binaries
3. packages Linux `.deb` and `.rpm` release assets
4. uploads release assets and `checksums.txt`
5. updates `rtcoder/homebrew-tap` when `HOMEBREW_TAP_TOKEN` is configured

## License

MIT
