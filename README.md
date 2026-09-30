# GHFS Skill & MCP Server

Two ways to let an AI assistant drive [GHFS (Go HTTP File Server)](https://github.com/mjpclab/go-http-file-server):

- **[Agent skills](#agent-skills)** — teach an agent to run GHFS and to talk to it over plain HTTP with the tools it already has. No extra process to run.
- **[MCP server](#mcp-server)** — an [MCP](https://modelcontextprotocol.io/) server exposing list, upload, mkdir, delete, and archive as tools, for clients that prefer a typed tool surface.

## Agent Skills

Starting a server and using one are usually done by different people, so they
are two skills:

- `skills/ghfs-client/` drives a running server over its HTTP API: listing,
  uploading, creating directories, deleting, and archiving — including the
  failure modes that silently report success.
- `skills/ghfs-server/` installs GHFS and starts an instance with the right
  permissions, listen address, and TLS.

Link the ones you need into a skills path your runtime loads, so they track the
repository instead of going stale:

```bash
ln -s "$PWD/skills/ghfs-client" ~/.claude/skills/ghfs-client
ln -s "$PWD/skills/ghfs-server" ~/.claude/skills/ghfs-server
```

The link target has to be absolute. Copy the directories instead if you would
rather pin a version, or point an agent straight at a `SKILL.md`.

## MCP Server

The Go sources live in `mcp/`.

### Features

- **List directories** — browse GHFS directories with sorting support
- **Upload files/directories** — upload single files, multiple files, or entire directory structures
- **Create directories** — create one or more directories, including nested paths
- **Delete files/directories** — remove files or directories recursively
- **Archive/download** — generate archive download URLs (tar/tgz/zip)
- **Dual transport** — supports both STDIO and HTTP (Streamable HTTP) modes

### Install

```bash
cd mcp && go install .
```

This puts `ghfs-mcp-server` in `$(go env GOPATH)/bin`, or `$GOBIN` if set.

Alternatively, build a binary in place:

```bash
cd mcp && go build -o ghfs-mcp-server .
```

### Usage

```
ghfs-mcp-server [options]

Options:
  -ghfs-url string   GHFS server base URL (default "http://localhost:8080")
  -mode string       Run mode: stdio or http (default "stdio")
  -addr string       HTTP listen address, only for http mode (default ":8080", or ":8443" with TLS)
  -cert string       TLS certificate file (enables HTTPS in http mode)
  -key string        TLS private key file (enables HTTPS in http mode)
  -debug             Enable debug logging of MCP messages
```

#### STDIO Mode (default)

```bash
ghfs-mcp-server -ghfs-url http://localhost:8080
```

#### HTTP Mode

```bash
ghfs-mcp-server -mode http -addr :9090 -ghfs-url http://localhost:8080
```

The HTTP endpoint is available at `http://localhost:9090/`.

#### HTTPS Mode

```bash
ghfs-mcp-server -mode http -addr :9443 -cert server.crt -key server.key -ghfs-url http://localhost:8080
```

The HTTPS endpoint is available at `https://localhost:9443/`.

### MCP Tools

#### `ghfs_list`

List directory contents.

| Parameter | Type   | Required | Description                          |
| --------- | ------ | -------- | ------------------------------------ |
| `path`    | string | Yes      | Directory path, e.g. `/` or `/docs/` |
| `sort`    | string | No       | Sort order, e.g. `/T`, `n`, `S`      |

#### `ghfs_upload`

Upload files or directory structures.

| Parameter          | Type     | Required | Description                                          |
| ------------------ | -------- | -------- | ---------------------------------------------------- |
| `path`             | string   | Yes      | Target directory path                                |
| `files`            | object[] | Yes      | Array of files to upload                             |
| `files[].filepath` | string   | Yes      | Relative path (e.g. `file.txt` or `subdir/file.txt`) |
| `files[].content`  | string   | Yes      | Base64-encoded file content                          |

Files with `/` in `filepath` are uploaded using GHFS `dirfile` mode, which creates missing intermediate directories. Creating them needs mkdir permission on the target path as well as upload; without it GHFS returns HTTP 500 and writes nothing.

#### `ghfs_mkdir`

Create directories.

| Parameter | Type     | Required | Description                                  |
| --------- | -------- | -------- | -------------------------------------------- |
| `path`    | string   | Yes      | Parent directory path                        |
| `names`   | string[] | Yes      | Directory names (supports nested like `a/b`) |

#### `ghfs_delete`

Delete files or directories (recursive).

| Parameter | Type     | Required | Description              |
| --------- | -------- | -------- | ------------------------ |
| `path`    | string   | Yes      | Parent directory path    |
| `names`   | string[] | Yes      | Names of items to delete |

#### `ghfs_archive`

Generate an archive download URL for files on the GHFS server.

| Parameter  | Type     | Required | Description                                     |
| ---------- | -------- | -------- | ----------------------------------------------- |
| `path`     | string   | Yes      | Directory path to archive                       |
| `format`   | string   | Yes      | Archive format: `tar`, `tgz`, or `zip`          |
| `names`    | string[] | No       | Specific items to include (omit for entire dir) |
| `filename` | string   | No       | Custom filename for the archive download        |

### Client Configuration

#### Claude Desktop

STDIO mode — add to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "ghfs": {
      "command": "/path/to/ghfs-mcp-server",
      "args": ["-ghfs-url", "http://localhost:8080"]
    }
  }
}
```

#### VS Code (Copilot)

config file is `.vscode/mcp.json` (project scope) or `~/.config/Code/User/mcp.json` (user scope).

STDIO mode:

```json
{
  "servers": {
    "ghfs": {
      "command": "/path/to/ghfs-mcp-server",
      "args": ["-ghfs-url", "http://localhost:8080"]
    }
  }
}
```

HTTP mode:

```json
{
  "servers": {
    "ghfs": {
      "type": "http",
      "url": "http://localhost:9090/"
    }
  }
}
```
