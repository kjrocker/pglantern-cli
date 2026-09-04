# lantern

`gh`-style command-line client for [pgLantern](https://pglantern.com).

A basic passthrough client that exists so that we don't have to `curl` the pgLantern endpoints directly.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/kjrocker/pglantern-cli/main/install.sh | bash
```

Downloads the release binary to `~/.local/bin`, but the destination can be overridden if you'd prefer:

```sh
curl -fsSL https://raw.githubusercontent.com/kjrocker/pglantern-cli/main/install.sh | bash -s -- --bin-dir /usr/local/bin
```

Installs as `lantern`, but the command name can be changed too — useful if that name is already taken:

```sh
curl -fsSL https://raw.githubusercontent.com/kjrocker/pglantern-cli/main/install.sh | bash -s -- --bin-name pglantern
```

From source, with [mise](https://mise.jdx.dev):

```sh
mise install
go build -o lantern .
```

## Login

No key required. With none configured, the CLI runs on the anonymous per-IP
tier and every command works out of the box. A key only raises the rate limits
— mint one in the pgLantern web UI under `/users/api-keys` and `lantern login`
to save it.

```sh
lantern login                       # prompts for the key, validates, saves
lantern login --with-token < key    # scriptable
lantern login --host https://pglantern.example.com
```

The key and host are stored in `~/.config/lantern/config.json`.

The API key and the host can be passed per-command with `--api-key` / `--host` flags, injected into the environment with `LANTERN_API_KEY` / `LANTERN_HOST`, or just use the configuration file. `lantern logout` deletes the file.

## Usage

```sh
lantern lists
lantern messages --list pgsql-hackers --limit 10
lantern messages get '<message-id>'     # raw Message-Id, straight from a table row
lantern messages open '<message-id>'    # jump to the upstream archive page (--site for pgLantern)
lantern messages thread '<message-id>'
lantern search vacuum full --committed --major 17
lantern search vacuum --sort sent_at --dir asc  # oldest matching first
lantern senders --sort messages --dir desc
lantern senders get 42
lantern threads vacuum --from 2024-01-01       # discussion threads, newest activity first
lantern threads --sort messages --dir desc     # busiest threads first
lantern attachments --sort size         # biggest first; also --sort date
lantern attachments patch 1234          # parsed patch summary
lantern commits --q "shared_buffers" --path src/backend/access/ --major 16
lantern commits --sort authored --dir asc      # oldest author date first
lantern commits get <sha>               # full 40-hex sha
lantern commits open <sha>              # jump to the upstream commit page (--site for pgLantern)
lantern commits thread <sha>            # the discussion behind a commit
lantern analytics messages --interval month --cumulative
lantern analytics top-senders -n 10 --from 2025-01-01
lantern analytics thread-sizes
lantern versions
lantern versions gucs 17                        # GUC catalog
lantern versions gucs 17 --changed-since 16     # what changed between majors
lantern activity src/backend/access/    # merged commit + thread activity
lantern imports --list pgsql-hackers
```

`threads` and `senders` take an optional positional query like `search`
(`--q` remains as an alias).

Every command renders a table by default, but accepts a `--json` argument:

```sh
lantern messages --limit 3 --json | jq '.data[].subject'
```

In a terminal, tables truncate long cells and page through `$LANTERN_PAGER`,
`$PAGER`, or `less -FRX` (`--no-pager` disables). Piped output is untruncated
and unpaged.

Fetch a known set of records instead of a page with `--id` (repeatable; `-`
reads newline-delimited ids from stdin), on `messages`, `commits`, and
`senders`:

```sh
lantern threads --json | jq -r '.data[].starter.message_id' | lantern messages --id -
```

Paginated commands print the next-page cursor to **stderr**

```
# next: --after g3QAAAAC...
```

`--all` follows cursors client-side (sequential requests, 100 rows per page)
up to a `--max` row ceiling, default 5000. Hitting the ceiling prints a
resume note on stderr with the cursor to continue from. With `--json`, `--all`
streams one raw page document per line. Not combinable with `--before` or
`--id`.

Exit codes are scriptable: `0` success, `2` usage error, `3` auth (401/403),
`4` not found (404), `1` everything else.

For anything the CLI doesn't wrap, there's an escape hatch:

```sh
lantern api /search --param q=vacuum --param limit=5
```

## Shell completion

```sh
lantern completion zsh > ~/.local/share/zsh/site-functions/_lantern
lantern completion bash|fish|powershell   # likewise
```

## Development

```sh
go test ./...
go vet ./...
```

## Releasing

Maintainer-facing. Releases are built and published from a local machine — the Go build is `CGO_ENABLED=0`, so the darwin archives cross-compile from Linux.

```sh
mise install                  # go + goreleaser
cp .env.example .env          # then fill in GITHUB_TOKEN
make release-dry              # build all four archives into dist/, publish nothing
make release TAG=v0.3.0       # test, tag, push, publish to GitHub
```

`GITHUB_TOKEN` is a GitHub personal access token with `contents:write` on `kjrocker/pglantern-cli`. The Makefile reads it from `.env`, which is gitignored — do not commit it, and do not put it in `.env.example`. `.env` is the source of truth and overrides any `GITHUB_TOKEN` already exported in your shell.

`make release` refuses to run on a dirty tree, without a `v`-prefixed `TAG`, or without the token.

## License

[MIT](LICENSE)
