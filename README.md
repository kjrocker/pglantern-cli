# horton

`gh`-style command-line client for [pgLantern](https://pglantern.com).

A basic passthrough client that exists so that we don't have to `curl` the pgLantern endpoints directly.

## Install

```sh
curl -fsSL https://codeberg.org/kehvyn/horton-cli/raw/branch/main/install.sh | bash
```

Downloads the release binary to `~/.local/bin`, but the destination can be overridden if you'd prefer:

```sh
curl -fsSL https://codeberg.org/kehvyn/horton-cli/raw/branch/main/install.sh | bash -s -- --bin-dir /usr/local/bin
```

If you already have Go (1.24+) globally configured:

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

The key and host are stored in `~/.config/horton/config.json`.

The API key and the host can be passed per-command with `--api-key` / `--host` flags, injected into the environment with `HORTON_API_KEY` / `HORTON_HOST`, or just use the configuration file. `horton logout` deletes the file.

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
horton threads --sort messages --dir desc     # busiest threads first
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

Every command renders a table by default, but accepts a `--json` argument:

```sh
horton messages --limit 3 --json | jq '.data[].subject'
```

Paginated commands print the next-page cursor to **stderr**

```
# next: --after g3QAAAAC...
```

For anything the CLI doesn't wrap, there's an escape hatch:

```sh
horton api /search --param q=vacuum --param limit=5
```

## Shell completion

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
