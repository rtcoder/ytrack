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
- inspecting and resolving YouTrack users
- creating YouTrack projects
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

### Ubuntu / Debian

Download the `.deb` package for your CPU architecture from GitHub Releases,
then install it with `apt`:

```sh
sudo apt install ./ytrack_<version>_linux_amd64.deb
```

Use `ytrack_<version>_linux_arm64.deb` instead on ARM64 systems.

### Fedora

Download the `.rpm` package for your CPU architecture from GitHub Releases,
then install it with `dnf`:

```sh
sudo dnf install ./ytrack_<version>_linux_amd64.rpm
```

Use `ytrack_<version>_linux_arm64.rpm` instead on ARM64 systems.

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

Or run the interactive local setup:

```sh
ytrack init
```

Create an issue:

```sh
ytrack issue create "Crash on save" "Steps to reproduce..."
```

Move an issue to another status:

```sh
ytrack issue status ART-123 Done
```

Show the current user:

```sh
ytrack user me
```

Create a project:

```sh
ytrack project create "Mobile App" --key MOB --leader me
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

Generated command reference is available in [`docs/commands.md`](docs/commands.md).

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
ytrack init
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
ytrack issue create <summary> --description-file <path>
ytrack issue create <summary> --description-file -
ytrack issue create <summary> [description] [--type <type>] [--assignee <user>] [--priority <priority>] [--version <version>]
ytrack issue list
ytrack issue list --state <value>
ytrack issue list --assigned-to <user>
ytrack issue show <issue-id>
ytrack issue edit <issue-id> [--title <title>] [--description <description>] [--type <type>] [--priority <priority>]
ytrack issue type <issue-id> <type>
ytrack issue priority <issue-id> <priority>
ytrack issue comment <issue-id> <text>
ytrack issue assign <issue-id> <user>
ytrack issue command <issue-id> <command>
ytrack issue close <issue-id>
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

Create an issue with metadata:

```sh
ytrack issue create "Crash on save" "Steps to reproduce..." --type Bug --assignee me --priority High --version v0.1.10
```

Create an issue with a longer description from a file or stdin:

```sh
ytrack issue create "Crash on save" --description-file bug.md
cat bug.md | ytrack issue create "Crash on save" --description-file -
```

List issues in the configured local project:

```sh
ytrack issue list
ytrack issue list --state Submitted
ytrack issue list --assigned-to me
```

Change issue status:

```sh
ytrack issue status ART-123 Done
ytrack issue status ART-123 "In Progress"
ytrack issue close ART-123
```

Show issue details:

```sh
ytrack issue show ART-123
```

Edit issue fields:

```sh
ytrack issue edit ART-123 --title "New title"
ytrack issue edit ART-123 -t "New title" -d "New description" --type Task --priority High
ytrack issue type ART-123 Task
ytrack issue priority ART-123 High
```

Comment, assign, or run a raw YouTrack command:

```sh
ytrack issue comment ART-123 "Looks good"
ytrack issue assign ART-123 me
ytrack issue command ART-123 "Priority Critical"
```

`issue status` sends a YouTrack command in this form:

```text
State <status>
```

### Users

```sh
ytrack user me
ytrack user list
ytrack user list --top <count>
ytrack user list --skip <count>
ytrack user find <query>
```

Show the current user for the configured token:

```sh
ytrack user me
```

List users:

```sh
ytrack user list
ytrack user list --top 20
```

Find one user by ID, `me`, login, name, or email:

```sh
ytrack user find me
ytrack user find 24-55
ytrack user find m.scott
```

If a search matches multiple users, `ytrack` prints an error with the matching
IDs and logins so you can run the command again with a more specific value.

### Projects

```sh
ytrack project create [title]
ytrack project create [title] --key <project-key> --leader <user>
ytrack project create [title] --key <project-key> --leader <user> --set-project-id
ytrack project create [title] --key <project-key> --leader <user> --template kanban
ytrack project create [title] --key <project-key> --leader <user> --template scrum
ytrack project list issues
ytrack project list issues --status <value>
ytrack project list issues --user <value>
ytrack project list issues --type <value>
ytrack project list issues --priority <value>
ytrack project list statuses
ytrack project list users
ytrack project list types
ytrack project list priorities
ytrack project list versions
```

Create a project with command-line arguments:

```sh
ytrack project create "Mobile App" --key MOB --leader me
ytrack project create "Mobile App" --key MOB --leader rtcoder
ytrack project create "Mobile App" --key MOB --leader 1-2
ytrack project create "Mobile App" --key MOB --leader me --set-project-id
```

The `--leader` value accepts the same references as `ytrack user find`: `me`, a
YouTrack user ID, login, name, or email. The command resolves it to the user ID
required by YouTrack before creating the project.

Use `--set-project-id` to save the newly created project ID to the local
`./.ytrack/config.json` for the current directory.

Run without arguments to answer prompts:

```text
$ ytrack project create
Set project name:
Mobile App
Set project key:
MOB
Set leader:
me
Set new project as local project_id? [y/N]
y
```

If the current directory already has a local `project_id`, interactive mode asks
before overwriting it:

```text
Local project_id is 0-1. Overwrite with 0-16? [y/N]
```

List data for the configured local project:

```sh
ytrack project list issues
ytrack project list issues --status Submitted
ytrack project list issues --user me
ytrack project list issues --type Bug
ytrack project list issues --priority Normal
ytrack project list statuses
ytrack project list users
ytrack project list types
ytrack project list priorities
ytrack project list versions
```

### JSON Output

Use `--json` before the command:

```sh
ytrack --json issue create "Crash on save"
ytrack --json issue create "Crash on save" "Steps to reproduce..."
ytrack --json issue show ART-123
ytrack --json issue edit ART-123 --title "New title"
ytrack --json issue comment ART-123 "Looks good"
ytrack --json issue command ART-123 "Priority Critical"
ytrack --json issue status ART-123 Done
ytrack --json user me
ytrack --json user list --top 20
ytrack --json project create "Mobile App" --key MOB --leader me
ytrack --json project list users
```

Without `--json`, successful commands print human-readable output:

```text
Created ART-123: "Crash on save"
url: https://youtrack.example.com/issue/ART-123
Updated ART-123 to Done
```

With `--json`, API-backed commands print the raw YouTrack response. API error
responses with `error_description` are printed as concise actionable messages.

### Shell Completion

Generate shell completion scripts with:

```sh
ytrack completion bash
ytrack completion zsh
ytrack completion fish
ytrack completion powershell
```

## Required Configuration

`issue create` requires:

- `url`
- `token`
- local `project_id`

`issue status` requires:

- `url`
- `token`

User commands require:

- `url`
- `token`

`project create` requires:

- `url`
- `token`
- a project name
- a project key
- a resolvable project leader

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
