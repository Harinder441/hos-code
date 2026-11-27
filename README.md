# hos-code

hos-code is a fast, local-first code reader and reviewer that runs in your browser. It is the Code view of
[hos](https://github.com/Harinder441/hos), and it also runs on its own.

It is a fork of [px0](https://github.com/px0-ai/px0) by Arpit Bhayani (MIT). The fork removes the AI editing features
and self-update, adds the hos bridge (Mark Snag, PR review into an hos topic), serves every asset from the binary,
and asks for a login on any network address.

- Starts in under a millisecond; the server uses about 20–30 MB of RAM (upstream measurements, [BENCHMARKS.md](BENCHMARKS.md))
- One static Go binary: no Electron, no Node, no CGO, no runtime downloads
- About 280 languages highlighted through Chroma

## Features

- Pull request review (`hos-code <pr-url>`): merge-base diffs, inline review comments, the commits in the PR, and stage, commit or push from the browser.
- Unpushed commits: read what your branch is ahead by, one file diff at a time, before you push.
- Fast navigation: fuzzy file search, symbol outline and workspace regex search.
- Remote use: run it on a server, VM or container and open it from your browser.
- Large files: opens 400,000-line files by drawing only the visible rows.
- Viewer: 14 themes, Markdown with Mermaid diagrams, CSV/TSV tables, an image inspector, and optional language servers for go-to-definition and hover.

## Install

From a release (macOS, Linux, BSD), once releases are published:

```bash
curl -fsSL https://raw.githubusercontent.com/Harinder441/hos-code/master/install.sh | sh
```

Each release archive holds the `hos-code` binary, `LICENSE`, `THIRD_PARTY_NOTICES.md` and this README, and is listed
with its SHA-256 in `checksums.txt`.

From source (Go 1.25+ and Node.js or Bun):

```bash
git clone https://github.com/Harinder441/hos-code.git
cd hos-code
make build
install -d ~/.local/bin && install hos-code ~/.local/bin/
```

## Usage

```bash
hos-code                                        # current directory
hos-code ~/workspace/project                    # a directory
hos-code main.go:42                             # a file at a line
hos-code https://github.com/owner/repo/pull/123 # a pull request
hos-code -host 0.0.0.0 -port 7777 ~/workspace   # reachable from other machines (asks for a login)
hos-code -base-path /rev-123/ ~/workspace       # behind a reverse proxy under a subpath
```

## Login on a network address

On `127.0.0.1` there is no login. On any other address (a Tailscale IP, `0.0.0.0`) every request needs the login
token. Set it with `HOS_CODE_LOGIN_TOKEN`, or let hos-code generate one: it prints a URL containing `?token=…`.
Opening that URL once sets a cookie for that port. Scripts can send `Authorization: Bearer <token>`. hos passes the
token when it starts hos-code and adds it to the links it gives you.

## Files and variables

- Settings and state: `~/.hos-code/` (or `$XDG_CONFIG_HOME/hos-code/`, `$XDG_STATE_HOME/hos-code/`). An existing
  `~/.px0/` is moved there on first start and a link is left at the old path.
- Browser preferences saved under `px0.*` keys move to `hos-code.*` on first load.
- Variables: `HOS_CODE_LOGIN_TOKEN`, `HOS_URL` and `HOS_CODE_TOKEN` (the hos bridge), `HOS_CODE_TELEMETRY=0`.
  The old names `HOS_PX0_TOKEN` and `PX0_*` are still read, with a deprecation note.
- Telemetry is off unless a PostHog key is supplied at build or run time; release builds include none.

## Development

```bash
make check   # web checks, bundle, fast Go tests
make test    # full Go test suite
make build   # ./hos-code
make dist    # release archives in dist/
```

See [AGENT.md](AGENT.md) for the inner loop and [docs/](docs/) for features and internals.

## License

[MIT](LICENSE). Copyright (c) 2026 Arpit Bhayani, and the hos authors for the changes in this fork. Bundled
components and their licences: [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
