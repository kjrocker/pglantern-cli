# horton

`gh`-style command-line client for the [Horton](https://codeberg.org/kehvyn/horton)
mailing-list-archive JSON API (`/api/v1`).

It is deliberately a **dumb API client**: it checks required arguments and flag
types, passes everything else through verbatim, and prints the server's error
message when the API rejects a request. Domain validation (date formats,
cursors, major names, bounds) lives on the server.

## Install

```sh
curl -fsSL https://codeberg.org/kehvyn/horton-cli/raw/branch/main/install.sh | bash
```

Drops a prebuilt binary (Linux or macOS, amd64 or arm64) into `~/.local/bin`
and changes no shell rc files — if that directory isn't on your `PATH`, the
script prints the `export` line to add yourself. To install somewhere else:

```sh
curl -fsSL https://codeberg.org/kehvyn/horton-cli/raw/branch/main/install.sh | bash -s -- --bin-dir /usr/local/bin
```

With Go 1.24+:

```sh
go install codeberg.org/kehvyn/horton-cli@latest
```

Or from source, with [mise](https://mise.jdx.dev):

```sh
mise install
go build -o horton .
```

## Login

API keys are minted in the Horton web UI under `/users/api-keys`.

```sh
horton login                       # prompts for the key, validates, saves
horton login --with-token < key    # scriptable
horton login --host https://horton.example.com
```

The key and host are stored in `~/.config/horton/config.json` (mode 0600).
Overrides, highest first: `--api-key` / `--host` flags, then `HORTON_API_KEY` /
`HORTON_HOST`, then the config file. `horton logout` deletes the file.

## Usage

```sh
horton lists
horton messages --list pgsql-hackers --limit 10
horton messages get '<message-id>'     # raw Message-Id, straight from a table row
horton messages thread '<message-id>'
horton search vacuum full --committed --major 17
horton senders --sort messages --dir desc
horton senders get 42
horton threads --q vacuum --from 2024-01-01   # discussion threads, newest activity first
horton attachments patch 1234          # parsed patch summary
horton commits --path src/backend/access/ --major 16
horton commits get <sha>               # full 40-hex sha
horton commits thread <sha>            # the discussion behind a commit
horton versions
horton versions gucs 17                        # GUC catalog
horton versions gucs 17 --changed-since 16     # what changed between majors
horton activity src/backend/access/    # merged commit + thread activity
horton imports --list pgsql-hackers
```

Message commands take the raw Message-Id exactly as tables print it (a
surrounding `<>` from a mail header is fine) — the CLI percent-encodes it into
a single URL path segment for you.

Every command renders a human table by default; add `--json` for the raw
response body (indented on a terminal, compact through a pipe):

```sh
horton messages --limit 3 --json | jq '.data[].subject'
```

Paginated commands print the next-page cursor to **stderr** so tables stay
pipe-clean:

```
# next: --after g3QAAAAC...
```

For anything the CLI doesn't wrap, there's an escape hatch:

```sh
horton api /search --param q=vacuum --param limit=5
```

## Shell completion

Cobra generates completions:

```sh
horton completion zsh > ~/.local/share/zsh/site-functions/_horton
horton completion bash|fish|powershell   # likewise
```

## Development

```sh
go test ./...
go vet ./...
```

## License

[MIT](LICENSE)
