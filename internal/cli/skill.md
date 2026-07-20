---
name: query-lantern
description: >-
  Query the pgLantern mailing-list archive from the command line with the `lantern`
  CLI — messages, threads, senders, full-text search, postgres commits, patches,
  GUCs, and per-path activity. Use this whenever a question could be answered
  from the pgsql mailing-list archive or the postgres commit history — "when was
  X discussed", "who proposed Y", "what thread did this commit come from", "what
  GUCs changed between 16 and 17", "find the patch for Z", "search the archive
  for …", "what's happening in src/backend/…". Reach for this before
  hand-rolling curl against /api/v1 — the CLI already handles auth, Message-Id
  encoding, and pagination. Also use it when the user names the `lantern` command
  directly.
---

# Querying pgLantern

`lantern` is a `gh`-style command-line client for the pgLantern mailing-list-archive
JSON API (`/api/v1`). pgLantern indexes the pgsql mailing lists (pgsql-hackers,
-bugs, -performance) alongside the postgres commit history, and — this is the
interesting part — **links them**: a commit knows the thread it came from, a
thread knows the commits that landed from it.

The CLI is deliberately a **dumb, read-only client**. It checks required
arguments and flag types, passes everything else through verbatim, and prints
the server's error. Every request is a GET; nothing here can mutate the archive,
so you can explore freely.

## Setup

`lantern` is a single prebuilt binary. If it isn't already on your `PATH`,
install it:

```sh
curl -fsSL https://codeberg.org/kehvyn/pglantern-cli/raw/branch/main/install.sh | bash
```

That drops the binary into `~/.local/bin` and edits no shell rc files; if that
directory isn't on your `PATH`, the script prints the `export` line to add. With
Go 1.24+, `go install codeberg.org/kehvyn/pglantern-cli@latest` works too. Confirm
it's live with `lantern --version`.

Then authenticate. The CLI defaults to the hosted archive at
`https://pglantern.com`, so you only need a key. API keys are minted in the
pgLantern web UI under `/users/api-keys`; `lantern login` prompts for one, validates
it against the server, and saves the key + host to `~/.config/lantern/config.json`
(mode 0600):

```sh
lantern login                                           # hosted archive
lantern login --host https://your-self-hosted.example.com   # your own deployment
```

For non-interactive use, skip the config file and pass credentials per-invocation
via environment or flags:

```sh
export LANTERN_API_KEY=hml_…
export LANTERN_HOST=https://your-self-hosted.example.com   # omit for pglantern.com
```

Resolution order for both host and key: `--host`/`--api-key` flags, then
`$LANTERN_HOST` / `$LANTERN_API_KEY`, then the config file. Host falls back to the
hosted archive at `https://pglantern.com` if nothing else is set — so against a
self-hosted deployment you **must** supply the host, or every call targets
pglantern.com.

## Working as an agent: always use `--json`

Every command renders a human table by default. In a terminal, tables truncate
long fields (senders at 40 chars, subjects at 64) with an ellipsis and route
through a pager; piped output is untruncated and unpaged, so table output is
safe for `awk`/`cut`. Still prefer `--json` — it's the server's raw body,
shaped for `jq`, with nothing flattened:

```sh
lantern messages --limit 3 --json | jq -r '.data[].subject'
```

Responses come in two shapes: collections are `{"data":[…],"next_cursor":…}` and
single items are `{"data":{…}}`. The `.data` wrapper is easy to forget on `get`
commands — `jq '.data.thread_id'`, not `jq '.thread_id'`.

## Command map

Start from whichever entity the question is about:

| Question | Command |
|---|---|
| What lists exist? | `lantern lists` |
| Recent/filtered messages | `lantern messages --list pgsql-hackers --limit 10` |
| One message (+ body, attachments) | `lantern messages get '<message-id>'` |
| Open a message's archive page in a browser | `lantern messages open '<message-id>'` |
| Many messages with bodies, one request | `lantern messages --full --id -` |
| The whole thread around it | `lantern messages thread '<message-id>'` |
| Commits that landed from its thread | `lantern messages commits '<message-id>'` |
| Refs it cites (shas, paths, CVEs) | `lantern messages refs '<message-id>'` |
| Full-text search | `lantern search vacuum full --committed --major 17` |
| Discussion threads, newest activity | `lantern threads vacuum --from 2024-01-01` |
| Busiest threads first | `lantern threads --sort messages --dir desc` |
| People who post | `lantern senders --sort messages --dir desc` |
| Message volume over time | `lantern analytics messages --interval month --from 2024-01-01` |
| Community growth (new senders) | `lantern analytics senders --cumulative` |
| Most prolific senders (ranked) | `lantern analytics top-senders -n 10 --from 2025-01-01` |
| Thread size distribution | `lantern analytics thread-sizes` |
| One person + recent messages | `lantern senders get 42` |
| Commits by path/author/major | `lantern commits --path src/backend/access/ --major 16` |
| One commit (+ files, releases) | `lantern commits get <sha-or-prefix>` |
| Open a commit's archive page in a browser | `lantern commits open <sha-or-prefix>` |
| **The discussion behind a commit** | `lantern commits thread <sha>` |
| Attachments / parsed patch summary | `lantern attachments`, `lantern attachments patch 1234` |
| Majors and releases | `lantern versions` |
| GUC catalog / what changed | `lantern versions gucs 17 --changed-since 16` |
| Docs table of contents | `lantern versions docs 17` |
| Merged commit+thread timeline for a path | `lantern activity src/backend/access/` |
| Which mbox files were ingested | `lantern imports --list pgsql-hackers` |

Common filters on the list commands: `--from` / `--to` (ISO-8601 bounds),
`--limit`/`-n` (server default 25, **max 100** per page), `--after` /
`--before` (cursors), and `--all` to follow cursors client-side (see
Pagination). `threads` and `senders` take an optional positional query like
`search` (`lantern threads vacuum`, `lantern senders lane`; `--q` still works
as an alias, but not both at once) and share a sort vocabulary: `--sort
messages|first|last` with `--dir asc|desc` (server default `last`/`desc`;
leave both unset to keep it). `search` adds `--list`, `--sender`,
`--committed`, `--path`, `--major`, `--sort` (`relevance` default, or
`sent_at`). `senders` adds `--list` (scopes both the people and their stats to
that list). `commits` adds `--q` (substring over the commit message),
`--path`, `--author`, `--major`, and its own sort pair: `--sort
committed|authored` with `--dir asc|desc` (server default `committed`/`desc`;
under `--sort authored` the date column shows the author date instead).
`commits get` takes a full 40-hex sha **or**
any unambiguous prefix (≥ 4 hex, git-style); an ambiguous prefix errors and
asks for more characters.

`messages open` / `commits open` launch the entity's **upstream** archive page
(`postgr.es/m/…`, `postgr.es/c/…`) in the default browser; `--site` opens the
pgLantern page instead, and `--print` emits the resolved URL to stdout rather
than opening anything (use it in headless/SSH/CI). A Message-Id and a full
40-hex sha are turned into a URL locally with no request; only a **short sha
prefix** calls the API, to resolve it server-side.

The `analytics` subcommands are bounded aggregates — no cursors, no
pagination flags. The two series (`messages`, `senders`) take `--interval
year|quarter|month`, `--from`/`--to`, and `--cumulative` (running total);
`top-senders` takes `--limit`/`-n` and a window; `thread-sizes` takes a window
over the thread's `started_at`. Series rows are `bucket`, `bucket_start`,
`count` and are dense — empty buckets are present with `count: 0`.

For anything the CLI doesn't wrap, there's a GET escape hatch:

```sh
lantern api /search --param q=vacuum --param limit=5
```

## Message-Ids

Pass the **raw** Message-Id exactly as a table prints it. A surrounding `<>` from
a mail header is fine — the CLI strips it and percent-encodes the id into one
path segment for you. Do not pre-encode it, and quote it: Message-Ids routinely
contain `+`, `=`, and `$`, which the shell would otherwise mangle.

```sh
lantern messages get 'CAFiTN-sF_J8NB3xjie7g=2-R5v9aLqEE5jrtF2dMmwPngd9RBg@mail.gmail.com'
```

## Pagination

Collections are cursor-paginated. The cursor is printed to **stderr** so tables
stay pipe-clean:

```
# next: --after g3QAAAAC...
```

Don't scrape stderr — with `--json`, read `.next_cursor` (null on the last page)
and feed it back to `--after`:

```sh
C=$(lantern messages --limit 100 --json | jq -r '.next_cursor')
lantern messages --limit 100 --after "$C" --json | jq -r '.data[].subject'
```

### `--all`: follow cursors client-side

`--all` fetches page after page (sequentially, 100 rows per request unless
`--limit` says otherwise) until the collection runs out or the `--max` row
ceiling (default **5000**) is hit. There is no "unlimited" — a bigger dump
means passing a bigger `--max`. Hitting the ceiling prints a resume note on
stderr, so a capped run is never silently truncated:

```
# stopped at 5000 rows; resume with --after g3QAAAAC... or raise --max
```

With `--json`, `--all` **streams one raw page document per line** as it is
fetched (not one merged document), so pipe it through `jq` per-line:

```sh
lantern messages --all --max 1000 --json | jq -r '.data[].subject'
lantern messages --all --max 1000 --json | jq -s '[.[].data[]] | length'
```

Note `.next_cursor` on each streamed line is the *server's* per-page cursor —
for resuming, trust the stderr note, which fires only when rows were actually
left behind. `--all` composes with `--after` (start point) and every filter,
but not with `--before` or `--id` (usage error). For a full-corpus export,
don't: that's what the pipeline/DB is for.

### Exact ids

To fetch a known set of records instead of a page, pass `--id` (repeatable).
`-` reads newline-delimited ids from stdin, so one call's output feeds the next:

```sh
lantern threads --json | jq -r '.data[].starter.message_id' | lantern messages --id -
lantern search vacuum --json | jq -r '.data[].message_id' | lantern messages --id -
lantern messages --id '<pinned@host>' --id -        # explicit ids merge with piped ones
lantern senders --id 1 --id 2 --sort messages --dir desc
```

Constraints:

- Only `messages` (Message-Id), `commits` (**full 40-hex sha**, no prefix
  resolution), and `senders` (integer id). `threads` and `lists` don't have it.
- Max 100 ids, counted after de-duplication.
- Not combinable with `--limit`, `--after`, or `--before` — the CLI rejects that
  locally, and the result isn't paginated (both cursors are null).
- Filters and `--sort`/`--dir` still apply, ANDed with the id set.
- Ids that don't exist are silently dropped — you get the subset that does, not
  a 404. A short result means some ids missed, not an error. Because misses drop
  out, a positional zip is only safe when the count came back as you sent it;
  otherwise join on `.message_id` / `.sha` / `.id`.
- **Results come back in the order you supplied the ids**, so you can zip the
  output against your input. Passing `--sort`/`--dir` overrides that and sorts
  normally.
- `messages --id` accepts `--full`, so one request gets you bodies (see below).

## Gotchas

Verified against a live server. A few of the old traps here were API bugs that
have since been fixed — collection rows now carry a hydrated `sender` object and
`lists` array, `senders get` embeds the same full message shape as `search`, and
bad values fail loudly (below) instead of silently. What remains:

- **Bad values now fail loudly — but confirm empty *filtered* results.** Enum
  flags the CLI itself checks (`--dir`, `--sort` on `threads` and `senders`)
  print `must be one of: …` before any request. Server-side, a bad `--limit`
  (> 100), malformed
  date (`--from`/`--to`), unknown `search --sort`, or nonexistent `--major` all
  return a 422 error — no more silent fallback to relevance order or an empty
  table masquerading as "nothing landed." For `--limit`, dates, and `--major`
  the message names the param and its rule; for `search --sort` it's a terser
  `Invalid value for enum` (the offending param rides in `details`). Take major
  names from `lantern versions` (`16`, `9.6`, `master`). Still: a *filtered* query
  that returns empty is a real (valid) empty — re-run without the filter to tell
  a genuine absence from an over-narrow one.
- **`commits thread` writes its misses to stderr**: `no archived discussion
  found`, and `# unresolved ref (…)` for Message-Ids a commit trailer cites that
  aren't in the archive. A commit having no thread is normal and not an error —
  the exit code stays 0.
- **Patch attachments are sparse.** Most attachments aren't patches, so
  `attachments --limit 25` may show none; filter with
  `jq '[.data[]|select(.is_patch)]'` and page if you need one.
- **`versions` rows are mostly empty for unreleased majors** (no released/EOL
  date, no tag) — that's real data, not a broken query.
- **The per-major GUC catalog reflects a *late minor* of that major, not its
  `.0` release.** Postgres backpatches some GUCs into minor releases, and those
  show up in the older major's catalog here. `restrict_nonsystem_relation_kind`
  landed in 17.0 *and* was backpatched to 16.4, so pgLantern lists it under 16 and
  `gucs 17 --changed-since 16` does **not** call it added. A hand-rolled
  `REL_16_0` vs `REL_17_0` source diff *does*. Neither is wrong — they answer
  different questions ("new since 16.0" vs "new versus current 16.x"). The
  catalog response reports which minor it was snapshotted from in
  `data.server_version` (e.g. `"17.10 …"`), so cite that when it matters. For an
  upgrade audit, pgLantern's framing is usually the one you want, since you're
  upgrading from a patched 16, not from 16.0. Say which you mean when reporting.

## Recipes

**"What was the discussion behind this commit?"** — the linkage is the whole
point of the archive:

```sh
lantern commits thread 51cd5d6f052306e9288ff8c162ca9596432a5d2e
```

**"Did this proposal ever land?"** — search only threads with a landed commit,
then walk to the commits:

```sh
lantern search 'conflict log history' --committed --json | jq -r '.data[0].message_id'
lantern messages commits '<that-message-id>'
```

**"Who are the most prolific hackers?"** — the senders index carries stats
(`message_count`, `first_message_at`, `last_message_at`), unlike messages:

```sh
lantern senders --sort messages --dir desc --limit 10 --json \
  | jq -r '.data[] | "\(.stats.message_count)\t\(.email)"'
```

**"What changed in this GUC between majors?"** — the diff splits into added,
removed, default-changed, and context-changed sections:

```sh
lantern versions gucs 17 --changed-since 16 --json | jq '.data.added[].name'
```

**"What's been happening in this subsystem?"** — `activity` merges commits and
threads into one time-ordered feed, each row tagged `kind: commit|message`:

```sh
lantern activity src/backend/access/ --major 17 --limit 50 --json \
  | jq -r '.data[] | "\(.activity_at)\t\(.kind)\t\(.subject)"'
```

**"Read the opening post of the busiest threads"** — sort threads by size, pull
each thread's starter Message-Id, pipe the ids in with `--full`. Two commands,
one request each; the bodies come back in the order the ids went in:

```sh
lantern threads --sort messages --dir desc --limit 5 --json \
  | jq -r '.data[] | select(.starter.message_id) | .starter.message_id' \
  | lantern messages --full --id - --json \
  | jq -r '.data[] | "=== \(.subject)\n\(.sender.email)\t\(.sent_at)\n\n\(.body_text)\n"'
```

`--full` asks for the single-message shape (`body_text`, `attachments`) on every
collection row, so the rows are identical to what `messages get` returns — there
is no reason to loop. It works on `messages`, `search`, and `messages thread`.
Without `--json` it prints each message as a `messages get`-style block instead
of the summary table.

The `select(.starter.message_id)` guard matters: threads whose opening message
was never ingested have a null starter, and an empty id would otherwise become a
malformed request.

Drop `--full` when the summary fields are enough — the payload is much smaller:

```sh
lantern threads --sort messages --dir desc --limit 20 --json \
  | jq -r '.data[] | select(.starter.message_id) | .starter.message_id' \
  | lantern messages --id - --json \
  | jq -r '.data[] | "\(.sent_at)\t\(.sender.email)\t\(.subject)"'
```

## Exit codes

Scriptable, `curl`-style:

| Code | Meaning |
|---|---|
| 0 | success (including valid-but-empty results) |
| 1 | anything else: transport errors, 5xx, 422s the server rejected |
| 2 | usage: bad/conflicting flags, missing arguments, malformed ids |
| 3 | auth: 401/403 — no key, or the server rejected it |
| 4 | not found: 404 — the resource doesn't exist |

So `lantern commits get <sha> || ...` can distinguish "no such commit" (4)
from "bad key" (3) without parsing stderr.

## Pager

In a terminal, table output pages through `$LANTERN_PAGER`, `$PAGER`, or
`less -FRX` (which exits immediately if the output fits one screen). Piped
output never pages, so scripts and agents are unaffected; `--no-pager` forces
it off in a terminal.

## Troubleshooting

- `no API key: run 'lantern login'…` — nothing resolved a key; run `lantern login`
  or export `$LANTERN_API_KEY` (see Setup).
- `A valid API key is required.` — a key was sent and the **server** rejected it.
  Mint a fresh one in the web UI under `/users/api-keys` and `lantern login` again.
- `connection refused` / dial errors — nothing is listening at the host you
  targeted. With no host set the CLI defaults to the hosted archive at
  `https://pglantern.com`; if you're pointing at a self-hosted deployment,
  confirm `--host` / `$LANTERN_HOST` is set correctly.
