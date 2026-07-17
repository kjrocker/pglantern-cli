# Note: surface `error.details` (and give `search --sort` a client-side enum)

Status: **DONE** — both follow-ups landed. `DecodeError` now captures
`error.details` and `Error.Error()` appends `(parameter: X)` for the terse
schema-shaped 422s (`internal/api/client.go`), and `search --sort` validates
`relevance`/`sent_at` at parse time via `addEnumFlag` (`internal/cli/search.go`).
Kept below as the rationale record.

The server side of the pgsql-archive "rough edges" work landed in the `pgml-api`
repo (see its `GOTCHAS.md`): collection endpoints now hydrate `sender`/`lists`,
`search`'s `sort` and every `major` filter reject bad input with a 422, and the
422 messages name the offending parameter and its rule. Two items in that list
were marked **"Related CLI fix"** because they live here, not in the API. This
note fleshes them out so whoever picks them up has the full picture.

Neither is a correctness bug — the CLI already surfaces the server's error and
exits non-zero. They're about not *throwing away* the machine-readable half of
that error, and about failing a beat earlier for a known-bad flag.

---

## 1. `DecodeError` drops `error.details`

### Current behavior

`internal/api/client.go` decodes only `code` and `message` from the envelope:

```go
// internal/api/client.go:36-49
func DecodeError(status int, body []byte) *Error {
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	...
}
```

`Error` (`client.go:21-25`) has no field for `details`, and `Error.Error()`
(`client.go:27-32`) returns `Message` verbatim. So whatever the server put in
`details` never reaches the user.

### Why it matters

The API emits two `details` shapes, both settled (`pgml-api`
`docs/error-handling.md`):

1. **Domain-validated params** — `FallbackController`. The `message` already
   names the param and rule, and `details` carries the machine key:

   ```json
   {"error":{"code":"invalid_params",
     "message":"The 'limit' parameter must be an integer between 1 and 100.",
     "details":{"param":"limit"}}}
   ```

   Covers `limit`, `from`, `to`, `major`. These read fine today — the *message*
   is self-describing — so `details` is a bonus (scriptable param name).

2. **Schema-validated params** — `OpenApiErrorRenderer`. Here the `message` is a
   terse summary and the param only appears in `details.errors[].path`:

   ```json
   {"error":{"code":"invalid_params",
     "message":"Invalid value for enum",
     "details":{"errors":[{"path":"/sort","reason":"Invalid value for enum"}]}}}
   ```

   This is the one that stings: `horton search foo --sort banana` currently
   prints just `Invalid value for enum`, and the fact that **`sort`** is the
   culprit is sitting in `details`, discarded. (Also hits `committed`, and any
   other CastAndValidate rejection.)

### Proposed change

Keep `Error` decoding lenient and shape-agnostic — the two `details` shapes
differ, and more may appear. Capture the raw object and expose a helper that
renders whichever shape is present.

```go
// client.go
type Error struct {
	Status  int
	Code    string
	Message string
	Details json.RawMessage // raw error.details, nil when absent
}

func DecodeError(status int, body []byte) *Error {
	var env struct {
		Error struct {
			Code    string          `json:"code"`
			Message string          `json:"message"`
			Details json.RawMessage `json:"details"`
		} `json:"error"`
	}
	e := &Error{Status: status}
	if json.Unmarshal(body, &env) == nil {
		e.Code = env.Error.Code
		e.Message = env.Error.Message
		e.Details = env.Error.Details
	}
	return e
}
```

Then make `Error.Error()` append the param(s) when the message doesn't already
carry them. Decode `Details` against both known shapes (try
`{"param": "..."}`, then `{"errors":[{"path","reason"}]}`); ignore anything that
matches neither so an unknown future shape degrades to today's behavior.

```go
func (e *Error) Error() string {
	msg := e.Message
	if msg == "" {
		msg = fmt.Sprintf("HTTP %d", e.Status)
	}
	if params := e.detailParams(); params != "" {
		msg += " (parameter: " + params + ")"
	}
	return msg
}
```

`detailParams()` returns `"limit"` for shape 1, `"sort"` (from the `/sort` path,
stripped of its leading slash; comma-joined if several) for shape 2, and `""`
when `Details` is nil or unrecognized. Guard against double-printing: shape 1's
message usually already quotes the param, so only append when it's not already
substring-present, or just append for shape 2 (the terse one) and leave shape 1
alone. Result:

```
horton search foo --sort banana
# => Invalid value for enum (parameter: sort)
```

### Tests

Extend `internal/api/client_test.go` (`TestDecodeError` already feeds an
"envelope with details" case at line 27 — assert the new field there) and
`TestErrorString` (`client_test.go:54`) with both `details` shapes, asserting
the rendered string names the param for the enum shape and doesn't duplicate it
for the domain shape. Round-trip a real `{"errors":[{"path":"/sort",...}]}`
body, not a hand-built `Error`, so the decode path is covered.

---

## 2. `search --sort` has no client-side enum

### Current behavior

`internal/cli/search.go:27` declares `sort` as a plain string:

```go
cmd.Flags().String("sort", "", "sort order: relevance (default) or sent_at")
```

The server now rejects unknown values (422), so a typo is no longer *silent* —
but the round-trip is wasteful and the error is the terse enum message above.
`senders` already validates its own `sort`/`dir` locally via `addEnumFlag`
(`internal/cli/helpers.go:97-120`, used at `senders.go:43-44`), which fails at
parse time with a clear `must be one of: relevance, sent_at`.

### Proposed change

Mirror `senders`. Swap the plain flag for the enum helper:

```go
// search.go
addEnumFlag(cmd, "sort", "sort order", "relevance", "sent_at")
```

`addEnumFlag` registers an `enumFlag` whose zero value is empty, and
`collectQuery` only sends flags the user actually `Changed` — so an omitted
`--sort` still sends nothing and the server defaults to relevance, exactly as
now. `--sort relevance` and `--sort sent_at` pass; anything else fails locally
before any HTTP call. This is defense-in-depth / faster feedback, not a
correctness fix (the server is authoritative either way).

No new test infra needed — the existing enum-flag behavior is exercised via
`senders`; a command-level test in `internal/cli` asserting `--sort banana`
errors without hitting the network would be nice-to-have.

---

## Not in scope for this note

The other GOTCHAS entries were **resolved on the API side** and need no CLI
change — the CLI passes them through and they now Just Work:

- `messages`/`search` hydrate `sender`/`lists`.
- `senders get` embeds the full message summary shape.
- `commits get` resolves unambiguous sha prefixes (400 `ambiguous_sha` /
  404 / 400 `invalid_sha`).
- The GUC catalog reports its provenance minor as `data.server_version`.

The `skill.md` gotchas section has already been condensed to match. See
`pgml-api`'s `GOTCHAS.md` for the full inventory and the server-side rationale.
