---
name: query-horton
description: >-
  Query the Horton mailing-list archive from the command line with the `horton`
  CLI — messages, threads, senders, full-text search, postgres commits, patches,
  GUCs, and per-path activity. Use this whenever a question could be answered
  from the pgsql mailing-list archive or the postgres commit history — "when was
  X discussed", "who proposed Y", "what thread did this commit come from", "what
  GUCs changed between 16 and 17", "find the patch for Z", "search the archive
  for …", "what's happening in src/backend/…". Reach for this before
  hand-rolling curl against /api/v1 — the CLI already handles auth, Message-Id
  encoding, and pagination. Also use it when the user names the `horton` command
  directly.
---

# Querying Horton

`horton` is a `gh`-style command-line client for the Horton mailing-list-archive
JSON API (`/api/v1`). Horton indexes the pgsql mailing lists (pgsql-hackers,
-bugs, -performance) alongside the postgres commit history, and — this is the
interesting part — **links them**: a commit knows the thread it came from, a
thread knows the commits that landed from it.

The CLI is deliberately a **dumb, read-only client**. It checks required
arguments and flag types, passes everything else through verbatim, and prints
the server's error. Every request is a GET; nothing here can mutate the archive,
so you can explore freely.

## Setup

`horton` is a single prebuilt binary. If it isn't already on your `PATH`,
install it:

```sh
curl -fsSL https://codeberg.org/kehvyn/horton-cli/raw/branch/main/install.sh | bash
```

That drops the binary into `~/.local/bin` and edits no shell rc files; if that
directory isn't on your `PATH`, the script prints the `export` line to add. With
Go 1.24+, `go install codeberg.org/kehvyn/horton-cli@latest` works too. Confirm
it's live with `horton --version`.

Then authenticate. The CLI defaults to the hosted archive at
`https://pglantern.com`, so you only need a key. API keys are minted in the
Horton web UI under `/users/api-keys`; `horton login` prompts for one, validates
it against the server, and saves the key + host to `~/.config/horton/config.json`
(mode 0600):

```sh
horton login                                           # hosted archive
horton login --host https://your-self-hosted.example.com   # your own deployment
```

For non-interactive use, skip the config file and pass credentials per-invocation
via environment or flags:

```sh
export HORTON_API_KEY=hml_…
export HORTON_HOST=https://your-self-hosted.example.com   # omit for pglantern.com
```

Resolution order for both host and key: `--host`/`--api-key` flags, then
`$HORTON_HOST` / `$HORTON_API_KEY`, then the config file. Host falls back to the
hosted archive at `https://pglantern.com` if nothing else is set — so against a
self-hosted deployment you **must** supply the host, or every call targets
pglantern.com.

## Working as an agent: always use `--json`

Every command renders a human table by default. Tables truncate fields (senders
at 32 chars, subjects at 64) with an ellipsis, so **parsing the table loses
data**. Add `--json` to get the server's raw body and pipe it to `jq`:

```sh
horton messages --limit 3 --json | jq -r '.data[].subject'
```

Responses come in two shapes: collections are `{"data":[…],"next_cursor":…}` and
single items are `{"data":{…}}`. The `.data` wrapper is easy to forget on `get`
commands — `jq '.data.thread_id'`, not `jq '.thread_id'`.

## Command map

Start from whichever entity the question is about:

| Question | Command |
|---|---|
| What lists exist? | `horton lists` |
| Recent/filtered messages | `horton messages --list pgsql-hackers --limit 10` |
| One message (+ body, attachments) | `horton messages get '<message-id>'` |
| The whole thread around it | `horton messages thread '<message-id>'` |
| Commits that landed from its thread | `horton messages commits '<message-id>'` |
| Refs it cites (shas, paths, CVEs) | `horton messages refs '<message-id>'` |
| Full-text search | `horton search vacuum full --committed --major 17` |
| Discussion threads, newest activity | `horton threads --q vacuum --from 2024-01-01` |
| People who post | `horton senders --sort messages --dir desc` |
| One person + recent messages | `horton senders get 42` |
| Commits by path/author/major | `horton commits --path src/backend/access/ --major 16` |
| One commit (+ files, releases) | `horton commits get <sha-or-prefix>` |
| **The discussion behind a commit** | `horton commits thread <sha>` |
| Attachments / parsed patch summary | `horton attachments`, `horton attachments patch 1234` |
| Majors and releases | `horton versions` |
| GUC catalog / what changed | `horton versions gucs 17 --changed-since 16` |
| Docs table of contents | `horton versions docs 17` |
| Merged commit+thread timeline for a path | `horton activity src/backend/access/` |
| Which mbox files were ingested | `horton imports --list pgsql-hackers` |

Common filters on the list commands: `--from` / `--to` (ISO-8601 bounds),
`--limit` (server default 25, **max 100**), `--after` / `--before` (cursors).
`search` adds `--sender`, `--committed`, `--path`, `--major`, `--sort`
(`relevance` default, or `sent_at`). `commits` adds `--path`, `--author`,
`--major`. `commits get` takes a full 40-hex sha **or** any unambiguous prefix
(≥ 4 hex, git-style); an ambiguous prefix errors and asks for more characters.

For anything the CLI doesn't wrap, there's a GET escape hatch:

```sh
horton api /search --param q=vacuum --param limit=5
```

## Message-Ids

Pass the **raw** Message-Id exactly as a table prints it. A surrounding `<>` from
a mail header is fine — the CLI strips it and percent-encodes the id into one
path segment for you. Do not pre-encode it, and quote it: Message-Ids routinely
contain `+`, `=`, and `$`, which the shell would otherwise mangle.

```sh
horton messages get 'CAFiTN-sF_J8NB3xjie7g=2-R5v9aLqEE5jrtF2dMmwPngd9RBg@mail.gmail.com'
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
C=$(horton messages --limit 100 --json | jq -r '.next_cursor')
horton messages --limit 100 --after "$C" --json | jq -r '.data[].subject'
```

## Gotchas

Verified against a live server. A few of the old traps here were API bugs that
have since been fixed — collection rows now carry a hydrated `sender` object and
`lists` array, `senders get` embeds the same full message shape as `search`, and
bad values fail loudly (below) instead of silently. What remains:

- **Bad values now fail loudly — but confirm empty *filtered* results.** Enum
  flags the CLI itself checks (`--dir`, `--sort` on `senders`) print `must be one
  of: …` before any request. Server-side, a bad `--limit` (> 100), malformed
  date (`--from`/`--to`), unknown `search --sort`, or nonexistent `--major` all
  return a 422 error — no more silent fallback to relevance order or an empty
  table masquerading as "nothing landed." For `--limit`, dates, and `--major`
  the message names the param and its rule; for `search --sort` it's a terser
  `Invalid value for enum` (the offending param rides in `details`). Take major
  names from `horton versions` (`16`, `9.6`, `master`). Still: a *filtered* query
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
  landed in 17.0 *and* was backpatched to 16.4, so Horton lists it under 16 and
  `gucs 17 --changed-since 16` does **not** call it added. A hand-rolled
  `REL_16_0` vs `REL_17_0` source diff *does*. Neither is wrong — they answer
  different questions ("new since 16.0" vs "new versus current 16.x"). The
  catalog response reports which minor it was snapshotted from in
  `data.server_version` (e.g. `"17.10 …"`), so cite that when it matters. For an
  upgrade audit, Horton's framing is usually the one you want, since you're
  upgrading from a patched 16, not from 16.0. Say which you mean when reporting.

## Recipes

**"What was the discussion behind this commit?"** — the linkage is the whole
point of the archive:

```sh
horton commits thread 51cd5d6f052306e9288ff8c162ca9596432a5d2e
```

**"Did this proposal ever land?"** — search only threads with a landed commit,
then walk to the commits:

```sh
horton search 'conflict log history' --committed --json | jq -r '.data[0].message_id'
horton messages commits '<that-message-id>'
```

**"Who are the most prolific hackers?"** — the senders index carries stats
(`message_count`, `first_message_at`, `last_message_at`), unlike messages:

```sh
horton senders --sort messages --dir desc --limit 10 --json \
  | jq -r '.data[] | "\(.stats.message_count)\t\(.email)"'
```

**"What changed in this GUC between majors?"** — the diff splits into added,
removed, default-changed, and context-changed sections:

```sh
horton versions gucs 17 --changed-since 16 --json | jq '.data.added[].name'
```

**"What's been happening in this subsystem?"** — `activity` merges commits and
threads into one time-ordered feed, each row tagged `kind: commit|message`:

```sh
horton activity src/backend/access/ --major 17 --limit 50 --json \
  | jq -r '.data[] | "\(.activity_at)\t\(.kind)\t\(.subject)"'
```

## Troubleshooting

- `no API key: run 'horton login'…` — nothing resolved a key; run `horton login`
  or export `$HORTON_API_KEY` (see Setup).
- `A valid API key is required.` — a key was sent and the **server** rejected it.
  Mint a fresh one in the web UI under `/users/api-keys` and `horton login` again.
- `connection refused` / dial errors — nothing is listening at the host you
  targeted. With no host set the CLI defaults to the hosted archive at
  `https://pglantern.com`; if you're pointing at a self-hosted deployment,
  confirm `--host` / `$HORTON_HOST` is set correctly.
