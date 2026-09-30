# AGENTS.md

Guidance for agents working in this repository.

## What this repo contains

Two independent ways to let an agent drive [GHFS](https://github.com/mjpclab/go-http-file-server):

- `skills/` — agent skills, plain Markdown, no build step. They are split because running a server and using one are usually done by different people:
  - `skills/ghfs-client/SKILL.md` — using a running GHFS over its HTTP API.
  - `skills/ghfs-server/SKILL.md` — installing GHFS and starting an instance.
- `mcp/` — a Go MCP server wrapping the same operations as `skills/ghfs-client/` as typed tools.

The skills and the MCP server are deliberately separate. `mcp/` covers the same ground as `skills/ghfs-client/`, so a change to how GHFS's HTTP API behaves usually needs to land in **both**. `skills/ghfs-client/` also lists which server flag enables each `can*` permission; keep that table in step with `skills/ghfs-server/`.

## Layout

```
skills/ghfs-client/SKILL.md   using a server
skills/ghfs-server/SKILL.md   installing and starting a server
mcp/go.mod                    module root
mcp/main.go                   entry point, flag parsing, stdio/http mode
mcp/server/ghfs.go            GHFS HTTP client
mcp/server/tools.go           MCP tool definitions
mcp/server/handler.go         tool call handlers
mcp/server/server.go          server construction, debug middleware
```

## Build

```bash
cd mcp && go build -o ghfs-mcp-server .
```

The module path is the bare `ghfs-mcp-server`, not a GitHub URL. It does not track the repository name, so leave it alone when the repo is renamed.

## Tests

`mcp/server/ghfs_test.go` is an **integration** test suite. It talks to a real GHFS at `http://localhost:8080` under `/ttt/` — there are no mocks and no fixtures. Without a live server every test fails with `connection refused`, which says nothing about the code.

Start one first, with every write capability enabled:

```bash
mkdir -p /tmp/ghfs-test/ttt
ghfs -r /tmp/ghfs-test -l 127.0.0.1:8080 -U --global-mkdir --global-delete -A &
cd mcp && go test ./...
```

GHFS is read-only by default. Omitting `-U --global-mkdir --global-delete -A` gives HTTP 400 on the write tests rather than a connection error.

## Conventions

- Every request to GHFS sends `Accept: application/json`; without it the server returns its HTML browser UI.
- `mkdir` and `delete` are urlencoded, `upload` is multipart. Sending the wrong encoding returns HTTP 200 and `{"success":true}` while doing nothing.
- Uploads with a `/` in the path use the `dirfile` form field, which creates intermediate directories. `file` strips the path entirely.

`skills/ghfs-client/SKILL.md` covers these in full; read it before touching `mcp/server/ghfs.go`.

## When changing tools

Adding or changing an MCP tool means touching three places: the definition in `tools.go`, the handler in `handler.go`, and its registration in `NewServer` in `server.go`. Update the tool table in `README.md` in the same change.
