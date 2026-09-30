---
name: ghfs-server
description: Use when installing GHFS (Go HTTP File Server), or when starting, restarting, or configuring a GHFS instance, including its write permissions, listen address, and HTTPS.
---

# Running a GHFS server

## Overview

GHFS serves a filesystem directory over HTTP. This skill covers getting the
binary and starting a server with the right permissions. Listing, uploading,
and deleting files on a running server are covered by the `ghfs-client` skill.

**GHFS is read-only by default.** Anything that should be writable has to be
enabled when the process starts, so decide what the server's users need before
launching it.

## Installing GHFS

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

## Starting a server

Each write capability has its own flag, enabled either everywhere or only
below one URL path:

| Capability | Everywhere | Only under a URL path |
|---|---|---|
| Upload | `-U` | `-u /ttt` |
| Create directory | `--global-mkdir` | `--mkdir /ttt` |
| Delete | `--global-delete` | `--delete /ttt` |
| Download as archive | `-A` | `--archive /ttt` |

A path given to a scoped flag is a URL path, relative to `-r`, and covers every
directory below it.

**An upload that creates subdirectories also needs mkdir.** With upload enabled
but mkdir not, a client can write files only into directories that already
exist; an upload into a new subdirectory fails with HTTP 500. Enable both for
any path where clients upload trees.

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

Confirm it is up, and that it grants what you meant it to, with a single
request:

```bash
curl -s -H 'Accept: application/json' http://127.0.0.1:8080/
```

The listing carries `canUpload`, `canMkdir`, `canDelete`, and `canArchive` for
that exact path. They vary per directory, so check a directory that is meant to
be writable, not only the root.

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
