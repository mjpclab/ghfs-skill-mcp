---
name: ghfs
description: Use when installing or starting a GHFS (Go HTTP File Server) instance, or when listing, uploading, downloading, creating, deleting, or archiving files on a local or remote GHFS server over HTTP.
---

# GHFS

## Overview

GHFS serves a filesystem directory over HTTP. Every operation targets the URL of
the directory it acts on and is selected by a query flag: `?upload`, `?mkdir`,
`?delete`, `?zip`. There is no separate API endpoint and no request path other
than the directory itself.

Two rules cover most mistakes:

- **Send `Accept: application/json` on every request.** Without it the server
  returns the HTML browser UI.
- **Match the body encoding to the operation.** `mkdir` and `delete` are
  urlencoded; `upload` is multipart. Getting this wrong reports success and does
  nothing.

## Part 1 — Installing GHFS

Check whether it is already there before installing anything:

```bash
command -v ghfs && ghfs --version
```

Three ways to install it, in order of preference.

### 1. go install

```bash
go install mjpclab.dev/ghfs@latest
```

The module path is `mjpclab.dev/ghfs`, not the GitHub URL. The binary lands in
`$(go env GOPATH)/bin`, which has to be on your `PATH`. Set `GOBIN` to put it
somewhere else, such as a directory you can write to without root:

```bash
GOBIN=/somewhere/bin go install mjpclab.dev/ghfs@latest
```

### 2. Build from source

```bash
git clone --depth 1 https://github.com/mjpclab/go-http-file-server.git
cd go-http-file-server
go build .
```

Both of these report `Version: dev`, because the real version is stamped in at
release time rather than compiled from the source tree. Run
`bash build/build-current.sh` instead if you want a versioned build; it writes a
release-style archive into `output/`.

### 3. Prebuilt binary

Last resort. Releases are at
<https://github.com/mjpclab/go-http-file-server/releases>, with assets named
`ghfs-<version>-<os>-<arch>.tar.gz`, or `.zip` for Windows. Builds cover macOS,
Linux, FreeBSD, and Windows across amd64, arm64, and several other
architectures.

**On macOS, prefer one of the first two methods.** The release binaries are
neither signed nor notarized, so Gatekeeper refuses to run them.

If a prebuilt binary is the only option on macOS, download it with `curl` rather
than through a browser. Browsers tag downloads with a quarantine attribute and
`curl` does not, so this avoids the problem instead of having to undo it.

A binary that is already quarantined makes macOS report that it cannot be opened
because the developer cannot be verified. **Fetching the same release asset
again with `curl` is the fix**, and it needs no special permission, because the
fresh copy is never tagged in the first place. Replace the blocked file with it.

Only when re-downloading is impossible does the attribute have to be cleared,
and **you do not clear it yourself**. Show the person this command and let them
run it:

```bash
xattr -d com.apple.quarantine /path/to/ghfs
```

Clearing quarantine is a decision to trust unsigned code off the internet. That
belongs to whoever owns the machine, not to the agent working on it.

## Part 2 — Starting a server locally

**GHFS is read-only by default.** Writing, deleting, and archiving each have to
be enabled explicitly when the process starts. An agent that starts a server
without these flags cannot upload to it afterwards.

| Capability | Everywhere | Only under a URL path |
|---|---|---|
| Upload | `-U` | `-u /ttt` |
| Create directory | `--global-mkdir` | `--mkdir /ttt` |
| Delete | `--global-delete` | `--delete /ttt` |
| Download as archive | `-A` | `--archive /ttt` |

Other flags worth knowing: `-r <dir>` sets the served root (default `.`),
`-l <addr:port>` sets the listen address, `-L -` writes the access log to stdout,
`--user name:password` with `--global-auth` turns on Basic Auth.

**The IP in `-l` decides who can reach the server.** A value without an IP, such
as `8080` or `:8080`, listens on every IPv4 and IPv6 interface, so other
machines can connect. `0.0.0.0` covers IPv4 only and `[::]` IPv6 only. For a
server only this machine should use, such as a scratch server, bind
`127.0.0.1:<port>`. Leave the IP out only when other machines are meant to
reach it.

**Always give a port.** Without one it uses 80, or 443 with TLS, and leaving
`-l` out entirely means `:80`. A non-root user usually cannot bind either.

A scratch server with everything enabled:

```bash
ghfs -r /path/to/serve -l 127.0.0.1:8080 -U --global-mkdir --global-delete -A -L - &
```

A safer shape, where only `/ttt` is writable and the rest is read-only:

```bash
ghfs -r /path/to/serve -l 127.0.0.1:8080 -u /ttt --mkdir /ttt --delete /ttt --archive /ttt &
```

Confirm it is up, and see what you are allowed to do, with a single request:

```bash
curl -s -H 'Accept: application/json' http://127.0.0.1:8080/
```

The listing carries `canUpload`, `canMkdir`, `canDelete`, and `canArchive` for
that exact path. Read them before attempting a write. They vary per directory,
so check the directory you intend to write to, not the root.

### Serving over HTTPS

Pass a certificate and key. `-l` then serves TLS on that port, and a plain HTTP
request to it gets HTTP 400. `--listen-tls` forces TLS and `--listen-plain`
forces cleartext even when certs are supplied.

```bash
ghfs -r /path/to/serve --listen-tls 8443 \
  -c /path/to/server.crt -k /path/to/server.key &
```

This example gives no IP, so it listens on all interfaces. That is usually what
a server with a real certificate wants. For a local-only TLS server, use
`--listen-tls 127.0.0.1:8443`.

Reach it by the hostname the certificate is issued for, so verification passes
without `-k`.

To use a server someone else started, find it with
`ps -eo pid,args | grep ghfs`. A system instance is often driven by a config
file such as `/etc/ghfs.conf`, one flag per line.

## Part 3 — Talking to a server over HTTP

Everything below works the same against a local or a remote instance. Only the
base URL changes.

```bash
BASE=http://127.0.0.1:8080
```

| Operation | Request |
|---|---|
| List a directory | `GET <dir>/` with `Accept: application/json` |
| Sort a listing | add `?sort=<key>` |
| Download one file | `GET <file>` with **no** `Accept: application/json` |
| Create directories | `POST <dir>/?mkdir`, urlencoded, repeat `name=` |
| Upload files | `POST <dir>/?upload`, multipart |
| Delete | `POST <dir>/?delete`, urlencoded, repeat `name=` |
| Download an archive | `GET <dir>/?zip`, `?tar`, or `?tgz` |

### List

```bash
curl -s -H 'Accept: application/json' "$BASE/reports/"
```

Returns `subItems`, each with `name`, `isDir`, `size`, and `modTime`, plus the
`can*` permission flags for that directory.

Sort keys: `n` name, `e` extension, `s` size, `t` time, `_` unsorted. Uppercase
reverses. A leading `/` puts directories first, a trailing `/` puts them last.
So `?sort=/T` means directories first, newest first.

### Create directories

Urlencoded, one `name` per directory. Nested paths are allowed. `curl --data`
sets `Content-Type: application/x-www-form-urlencoded` for you; set it yourself
if you are not using curl.

```bash
curl -s -H 'Accept: application/json' -X POST \
  --data 'name=reports&name=archive/2024' "$BASE/?mkdir"
```

### Upload

Multipart. **The form field name decides how the filename is interpreted**, and
this is the single most common thing to get wrong.

**When the filename contains a `/`, you almost always want `dirfile`.** It is
the only field that reproduces the path as written.

Posting the same filename `L1/L2/f.txt` under each field name:

| Field | Result | Meaning |
|---|---|---|
| `dirfile` | `L1/L2/f.txt` | keeps the whole path, creates missing directories |
| `file` | `f.txt` | drops **every** directory, keeps the basename only |
| `innerdirfile` | `L2/f.txt` | drops only the **first** segment, keeps the rest |

Use `file` for flat uploads where the name has no `/` at all. Reach for
`innerdirfile` only when you deliberately want a tree stripped of its top-level
folder.

```bash
# flat file into /reports/
curl -s -H 'Accept: application/json' -X POST \
  -F 'file=@notes.txt;filename=notes.txt' "$BASE/reports/?upload"

# creates /reports/2024/ automatically in the same request
curl -s -H 'Accept: application/json' -X POST \
  -F 'dirfile=@q1.txt;filename=2024/q1.txt' "$BASE/reports/?upload"
```

Post to a directory that already exists and encode the new subdirectories in the
filename. Posting to a URL whose directory does not exist yet returns HTTP 400,
whatever field name you use.

One request may carry many parts, and may mix `file` and `dirfile`. Uploading a
name that already exists overwrites it silently. Binary content round-trips
byte for byte.

### Delete

Urlencoded, like `mkdir`. Directories are removed recursively.

```bash
curl -s -H 'Accept: application/json' -X POST \
  --data 'name=notes.txt&name=olddir' "$BASE/reports/?delete"
```

### Archive

A plain GET that streams the archive. `?zip`, `?tar`, and `?tgz` are the three
formats. Add `name=` parameters to archive only certain entries, and give the
flag a value to set the download filename.

```bash
curl -s -o reports.zip "$BASE/reports/?zip"
curl -s -o q1.zip      "$BASE/reports/?zip=q1.zip&name=2024"
```

## Confirming an operation actually happened

GHFS reports failure inconsistently, so trust the HTTP status and a follow-up
listing rather than the response body.

- A successful write returns `{"success":true}`.
- **A `mkdir` or `delete` sent as multipart instead of urlencoded also returns
  HTTP 200 and `{"success":true}`, and does nothing at all.** Checking the body
  will not catch this. Use `--data`, never `-F`, for these two operations.
- A denied operation returns HTTP 400. For `upload`, `mkdir`, and `delete` the
  body is an ordinary directory listing with no `success` field in it; a denied
  archive returns HTML. Neither shape contains an error message worth parsing.
- Deleting a name that does not exist returns `{"success":true}`.

After any write, re-list the directory and confirm the expected entry.

## Browser mode

The same URLs without `Accept: application/json` return the HTML file browser a
human would use. That is worth opening when someone wants to eyeball a tree or
hand a link to a person.

Prefer JSON for everything an agent does. It is the data itself rather than a
page to parse, and it is far smaller: one test directory rendered as 5145 bytes
of HTML and 782 bytes of JSON.

## Common mistakes

| Mistake | What happens | Fix |
|---|---|---|
| `?json` query parameter | HTML comes back; there is no such flag | `Accept: application/json` header |
| `-F name=x` for mkdir or delete | HTTP 200, `{"success":true}`, nothing created | `--data 'name=x'` |
| `file` field with a `/` in the filename | path stripped, file lands flat | use `dirfile` |
| `innerdirfile` to keep the full path | first path segment silently dropped | use `dirfile` |
| POST to a not-yet-existing directory | HTTP 400 | POST to the parent, put the subpath in the filename |
| Treating HTTP 400 as a bad request to retry | it usually means the operation is not permitted there | check `can*` in the listing, restart the server with the right flag |
| `Accept: application/json` when downloading a file | returns the file's metadata, not its content | omit the header for file downloads |
