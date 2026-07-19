# lantern

`gh`-style command-line client for [pgLantern](https://pglantern.com).

A basic passthrough client that exists so that we don't have to `curl` the pgLantern endpoints directly.

## Install

```sh
curl -fsSL https://codeberg.org/kehvyn/pglantern-cli/raw/branch/main/install.sh | bash
```

Downloads the release binary to `~/.local/bin`, but the destination can be overridden if you'd prefer:

```sh
curl -fsSL https://codeberg.org/kehvyn/pglantern-cli/raw/branch/main/install.sh | bash -s -- --bin-dir /usr/local/bin
```

Installs as `lantern`, but the command name can be changed too — useful if that name is already taken:

```sh
curl -fsSL https://codeberg.org/kehvyn/pglantern-cli/raw/branch/main/install.sh | bash -s -- --bin-name pglantern
```

If you already have Go (1.24+) globally configured:

```sh
go install codeberg.org/kehvyn/pglantern-cli@latest
```

Or from source, with [mise](https://mise.jdx.dev):

```sh
mise install
go build -o lantern .
```

## Login

API keys are minted in the pgLantern web UI under `/users/api-keys`.

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
lantern messages thread '<message-id>'
lantern search vacuum full --committed --major 17
lantern senders --sort messages --dir desc
lantern senders get 42
lantern threads --q vacuum --from 2024-01-01   # discussion threads, newest activity first
lantern threads --sort messages --dir desc     # busiest threads first
lantern attachments patch 1234          # parsed patch summary
lantern commits --path src/backend/access/ --major 16
lantern commits get <sha>               # full 40-hex sha
lantern commits thread <sha>            # the discussion behind a commit
lantern versions
lantern versions gucs 17                        # GUC catalog
lantern versions gucs 17 --changed-since 16     # what changed between majors
lantern activity src/backend/access/    # merged commit + thread activity
lantern imports --list pgsql-hackers
```

Every command renders a table by default, but accepts a `--json` argument:

```sh
lantern messages --limit 3 --json | jq '.data[].subject'
```

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
cp .env.example .env          # then fill in GITEA_TOKEN
make release-dry              # build all four archives into dist/, publish nothing
make release TAG=v0.3.0       # test, tag, push, publish to Codeberg
```

`GITEA_TOKEN` is a Codeberg access token scoped `write:repository`. The Makefile reads it from `.env`, which is gitignored — do not commit it, and do not put it in `.env.example`. `.env` is the source of truth and overrides any `GITEA_TOKEN` already exported in your shell.

`make release` refuses to run on a dirty tree, without a `v`-prefixed `TAG`, or without the token.

## License

[MIT](LICENSE)
