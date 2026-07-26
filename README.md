# ytrack

Cross-platform CLI for managing YouTrack issues from your terminal, with global
and per-project configuration.

## Status

`ytrack` is an early MVP. It supports:

- global YouTrack URL/token configuration
- local per-directory URL/token/project configuration in `./.ytrack/config.json`
- creating issues
- changing issue status through YouTrack commands
- raw JSON output for API-backed commands

## Install

From this repository:

```sh
go build -o ytrack ./cmd/ytrack
```

Or install into your Go bin directory:

```sh
go install ./cmd/ytrack
```

## Configuration

Configuration is merged at runtime with this priority:

```text
local ./.ytrack/config.json > global OS user config
```

The global config file lives under Go's `os.UserConfigDir()`:

- macOS: typically `~/Library/Application Support/ytrack/config.json`
- Linux: typically `~/.config/ytrack/config.json`
- Windows: typically `%APPDATA%\ytrack\config.json`

The local config file lives in the current working directory:

```text
./.ytrack/config.json
```

`project_id` is local-only by design. It is never read from or written to the
global config.

### Global Config

```sh
ytrack global set-url https://youtrack.example.com
ytrack global set-token perm:your-token
ytrack global show
ytrack global unset-url
ytrack global unset-token
```

### Local Config

```sh
ytrack set-url https://youtrack.example.com
ytrack set-token perm:project-specific-token
ytrack set-project-id 0-1
ytrack show
ytrack unset-url
ytrack unset-token
ytrack unset-project-id
```

`show` prints the effective merged config. Tokens are masked, for example:

```text
token: perm:xxxx...abcd
```

Config files containing tokens are written with `0600` permissions where the
operating system supports POSIX file modes.

Add this to repositories where you use local config:

```gitignore
.ytrack/
```

## Usage

Create an issue:

```sh
ytrack issue create "Crash on save" "Steps to reproduce..."
```

This sends:

```text
POST {url}/api/issues?fields=id,idReadable,summary
```

and prints a human-readable result:

```text
Created ART-123: "Crash on save"
```

Change issue status:

```sh
ytrack issue status ART-123 Done
```

This sends a YouTrack command:

```text
State Done
```

against the given issue.

Use raw JSON output for scripting:

```sh
ytrack --json issue create "Crash on save"
ytrack --json issue status ART-123 Done
```

## Required Configuration

`issue create` requires:

- `url`
- `token`
- local `project_id`

`issue status` requires:

- `url`
- `token`

Missing values produce actionable errors such as:

```text
missing configured project_id, run `ytrack set-project-id <project-id>`
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
