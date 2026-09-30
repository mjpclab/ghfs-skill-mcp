---
name: ghfs-client
description: Use when listing, uploading, downloading, creating, deleting, or archiving files on a local or remote GHFS (Go HTTP File Server) instance over HTTP.
---

# Using a GHFS server

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

## Finding the server and what it allows

Use the base URL you were given. For a server already running on this machine,
`ps -eo pid,args | grep ghfs` shows its listen address in the `-l` or
`--listen-tls` argument; with neither, it is on port 80. Starting or installing a server is covered by the
`ghfs-server` skill.

Every listing carries `canUpload`, `canMkdir`, `canDelete`, and `canArchive` for
that exact directory. **Read them before attempting a write.** They vary per
directory, so check the directory you intend to write to, not the root.

Scoped flags cover the whole subtree under their path. A `false` here is fixed
only by restarting the server with a different flag. It
cannot be worked around with a different request. Tell whoever runs the server
which flag is missing:

| Flag in listing | Server needs |
|---|---|
| `canUpload` | `-U`, or `-u <path>` |
| `canMkdir` | `--global-mkdir`, or `--mkdir <path>` |
| `canDelete` | `--global-delete`, or `--delete <path>` |
| `canArchive` | `-A`, or `--archive <path>` |

For an HTTPS server, use the hostname the certificate is issued for, so
verification passes without `curl -k`. A plain `http://` request to a TLS port
gets HTTP 400.

## Talking to a server over HTTP

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

# creates /reports/2024/ in the same request (needs canMkdir too)
curl -s -H 'Accept: application/json' -X POST \
  -F 'dirfile=@q1.txt;filename=2024/q1.txt' "$BASE/reports/?upload"
```

**Creating those directories needs `canMkdir` as well as `canUpload`.** Without
it, a `dirfile` upload whose path names a directory that does not exist yet
returns HTTP 500 with `{"success":false}` and writes nothing. A `dirfile` path
into directories that already exist needs only `canUpload`.

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
- A `dirfile` upload that would create a directory without mkdir permission
  returns HTTP 500 and `{"success":false}`.

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
| `dirfile` into new directories with `canMkdir` false | HTTP 500, nothing written | ask for mkdir to be enabled, or upload only into existing directories |
| POST to a not-yet-existing directory | HTTP 400 | POST to the parent, put the subpath in the filename |
| Treating HTTP 400 as a bad request to retry | it usually means the operation is not permitted there | check `can*` in the listing; if false, the server has to be restarted with the flag |
| `Accept: application/json` when downloading a file | returns the file's metadata, not its content | omit the header for file downloads |
