# ytrack command reference

## ytrack

Manage YouTrack issues from your terminal

```text
ytrack
```

## ytrack completion

Generate shell completion scripts

```text
ytrack completion bash|zsh|fish|powershell
```

## ytrack global

Manage global ytrack configuration

```text
ytrack global
```

## ytrack global set-token

Set global YouTrack token

```text
ytrack global set-token <token>
```

## ytrack global set-url

Set global YouTrack URL

```text
ytrack global set-url <url>
```

## ytrack global show

Show global configuration

```text
ytrack global show
```

## ytrack global unset-token

Unset global YouTrack token

```text
ytrack global unset-token
```

## ytrack global unset-url

Unset global YouTrack URL

```text
ytrack global unset-url
```

## ytrack init

Interactively configure ytrack for the current directory

```text
ytrack init
```

## ytrack issue

Manage YouTrack issues

```text
ytrack issue
```

## ytrack issue assign

Assign a YouTrack issue

```text
ytrack issue assign <issue-id> <user>
```

## ytrack issue close

Close a YouTrack issue as Fixed

```text
ytrack issue close <issue-id>
```

## ytrack issue command

Apply an arbitrary YouTrack command to an issue

```text
ytrack issue command <issue-id> <command>
```

## ytrack issue comment

Add a comment to a YouTrack issue

```text
ytrack issue comment <issue-id> <text>
```

## ytrack issue create

Create a YouTrack issue

```text
ytrack issue create <summary> [description] [flags]
```

## ytrack issue edit

Edit a YouTrack issue

```text
ytrack issue edit <issue-id> [flags]
```

## ytrack issue list

List issues in the configured project

```text
ytrack issue list [flags]
```

## ytrack issue priority

Set a YouTrack issue priority

```text
ytrack issue priority <issue-id> <value>
```

## ytrack issue show

Show a YouTrack issue

```text
ytrack issue show <issue-id>
```

## ytrack issue status

Set a YouTrack issue status

```text
ytrack issue status <issue-id> <status>
```

## ytrack issue type

Set a YouTrack issue type

```text
ytrack issue type <issue-id> <value>
```

## ytrack project

Manage YouTrack projects

```text
ytrack project
```

## ytrack project create

Create a YouTrack project

```text
ytrack project create [title] [flags]
```

## ytrack project list

List project issues and project metadata

```text
ytrack project list issues|statuses|users|types|priorities|versions [flags]
```

## ytrack set-project-id

Set local YouTrack project ID

```text
ytrack set-project-id <project-id>
```

## ytrack set-token

Set local YouTrack token

```text
ytrack set-token <token>
```

## ytrack set-url

Set local YouTrack URL

```text
ytrack set-url <url>
```

## ytrack show

Show effective configuration

```text
ytrack show
```

## ytrack unset-project-id

Unset local YouTrack project ID

```text
ytrack unset-project-id
```

## ytrack unset-token

Unset local YouTrack token

```text
ytrack unset-token
```

## ytrack unset-url

Unset local YouTrack URL

```text
ytrack unset-url
```

## ytrack user

Inspect YouTrack users

```text
ytrack user
```

## ytrack user find

Find a single YouTrack user

```text
ytrack user find <query>
```

## ytrack user list

List YouTrack users

```text
ytrack user list [flags]
```

## ytrack user me

Show the current YouTrack user

```text
ytrack user me
```

