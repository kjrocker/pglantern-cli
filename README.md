# lantern

`gh`-style command-line client for [pgLantern](https://pglantern.com).

A basic passthrough client that exists so that we don't have to `curl` the
pgLantern endpoints directly. Also provides terminal paging for large results.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/kjrocker/pglantern-cli/main/install.sh | bash
```

Downloads the release binary to `~/.local/bin`, but the destination can be
overridden if you'd prefer:

```sh
curl -fsSL https://raw.githubusercontent.com/kjrocker/pglantern-cli/main/install.sh | bash -s -- --bin-dir /usr/local/bin
```

Installs as `lantern`, but the command name can be changed too — useful if that
name is already taken:

```sh
curl -fsSL https://raw.githubusercontent.com/kjrocker/pglantern-cli/main/install.sh | bash -s -- --bin-name pglantern
```

From source, with [mise](https://mise.jdx.dev):

```sh
mise install
go build -o lantern .
```

## Upgrade

```sh
lantern upgrade               # latest release
lantern upgrade --check       # report installed vs latest, change nothing
lantern upgrade --to v0.9.0   # a specific release (downgrades too)
```

Downloads the release for your platform, verifies it against the release's
`checksums.txt`, and replaces the binary in place — wherever it was installed
and whatever it's named. If that's a root-owned directory like `/usr/local/bin`,
run it with `sudo`. Source builds report version `dev` and refuse unless you
pass `--force`.

## Login

We don't work with your authentication information, the client simply stores one
of your API keys.

If no API key is provided, you'll be on anonymous per-IP limits (which are quite
low). If you're self-hosting (or want to run behind a proxy or something), just
set `--host`.

```sh
lantern login                       # prompts for the key, validates, saves
lantern login --with-token < key    # scriptable
lantern login --host https://pglantern.example.com
```

The key and host are stored in `~/.config/lantern/config.json`.

The API key and the host can be passed per-command with `--api-key` / `--host`
flags, injected into the environment with `LANTERN_API_KEY` / `LANTERN_HOST`, or
just use the configuration file. `lantern logout` deletes the file.

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
lantern threads --list pgsql-hackers           # threads with a message on this list (stats stay whole-thread)
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
lantern watches                         # your saved alerts
lantern watches create --type query --q io_uring --list pgsql-hackers
lantern watches pause <id> / resume <id> / cadence <id> weekly / delete <id>
lantern watches endpoints               # your webhook delivery targets
```

`threads` and `senders` take an optional positional query like `search` (`--q`
remains as an alias).

Every command renders a table by default, but accepts a `--json` argument:

```sh
lantern messages --limit 3 --json | jq '.data[].subject'
```

In a terminal, tables truncate long cells and page through `$LANTERN_PAGER`,
`$PAGER`, or `less -FRX` (`--no-pager` disables).

Fetch a known set of records with `--id` (repeatable; `-` reads
newline-delimited ids from stdin), on `messages`, `commits`, and `senders`:

```sh
lantern threads --json | jq -r '.data[].starter.message_id' | lantern messages --id -
```

Paginated commands print the next-page cursor to **stderr**

```
# next: --after g3QAAAAC...
```

`--all` follows cursors client-side (sequential requests, 100 rows per page) up
to a `--max` row ceiling, default 5000. Hitting the ceiling prints a resume note
on stderr with the cursor to continue from. With `--json`, `--all` streams one
raw page document per line. Not combinable with `--before` or `--id`.

Exit codes are scriptable: `0` success, `2` usage error, `3` auth (401/403), `4`
not found (404), `1` everything else.

For anything the CLI doesn't wrap, there's an escape hatch:

```sh
lantern api /search --param q=vacuum --param limit=5
```

## Watches

Reads work with any key (or none — the anonymous tier). Watches are the one
write surface, and `create` / `pause` / `resume` / `cadence` / `delete` need a
key minted with **manage** access at `/users/api-keys`; a read key lists watches
fine but gets `403 key_read_only` (exit 3) on a mutation. `lantern api` stays
GET-only.

```sh
lantern watches                                     # ID TYPE PARAMS DELIVERY CADENCE STATUS
lantern watches get <id>
lantern watches create --type thread --message-id '<a1@example.com>'
lantern watches create --type sender --sender-id 42
lantern watches create --type path --path src/backend/access/ --major 17
lantern watches create --type guc --name work_mem
lantern watches create --type query --q io_uring --list pgsql-hackers --cadence weekly
lantern watches create --type query --q io_uring --channel webhook --endpoint <uuid>
lantern watches pause <id>            # stop delivering; resume puts it back
lantern watches cadence <id> weekly
lantern watches delete <id>           # no prompt; `# deleted <id>` on stderr
lantern watches endpoints             # ID URL STATUS
```

`--type` picks the subject flag: thread → `--message-id`, sender →
`--sender-id`, path → `--path` (+ `--major`), guc → `--name`, query → `--q` (+
`--list`, `--sender`). Those two checks — a valid `--type` and its subject flag
— are all the CLI validates; everything else is the server's 422.

Email watches carry a cadence: `daily` sends one email a day,
`weekly` one email every Monday covering everything since the last. Webhook
watches have no cadence and post to a `--endpoint` from
`lantern watches endpoints`.

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

Releases are built and published from my laptop — the Go build is
`CGO_ENABLED=0`, so the darwin archives cross-compile from Linux.

```sh
mise install                  # go + goreleaser
cp .env.example .env          # then fill in GITHUB_TOKEN
make release-dry              # build all four archives into dist/, publish nothing
make release TAG=v0.3.0       # test, tag, push, publish to GitHub
```

`GITHUB_TOKEN` is a GitHub token with `contents:write` on
`kjrocker/pglantern-cli`.

`make release` refuses to run on a dirty tree, without a `v`-prefixed `TAG`, or
without the token.

## License

[MIT](LICENSE)
