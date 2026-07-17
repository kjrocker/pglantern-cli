package api

// Response structs decode only the fields the table renderers show; --json
// bypasses these entirely and prints the raw body.

// Page is the keyset-paginated collection envelope. Non-paginated endpoints
// decode into it too — the cursors just stay nil.
type Page[T any] struct {
	Data       []T     `json:"data"`
	NextCursor *string `json:"next_cursor"`
	PrevCursor *string `json:"prev_cursor"`
}

// Item is the single-resource envelope.
type Item[T any] struct {
	Data T `json:"data"`
}

type List struct {
	Name      string `json:"name"`
	ShortDesc string `json:"short_desc"`
	Active    bool   `json:"active"`
}

type Sender struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

type MessageSummary struct {
	MessageID     string  `json:"message_id"`
	Subject       string  `json:"subject"`
	SentAt        string  `json:"sent_at"`
	FromRaw       string  `json:"from_raw"`
	HasAttachment bool    `json:"has_attachment"`
	ThreadID      string  `json:"thread_id"`
	Sender        *Sender `json:"sender"`
}

type AttachmentRef struct {
	ID          int    `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	IsPatch     bool   `json:"is_patch"`
}

type MessageFull struct {
	MessageSummary
	ToRaw       string          `json:"to_raw"`
	CcRaw       string          `json:"cc_raw"`
	BodyText    string          `json:"body_text"`
	Attachments []AttachmentRef `json:"attachments"`
}

type Ref struct {
	RefType     string  `json:"ref_type"`
	Value       string  `json:"value"`
	ResolvedSHA *string `json:"resolved_sha"`
	DocURL      string  `json:"doc_url"`
}

type Release struct {
	Branch   string `json:"branch"`
	FirstTag string `json:"first_tag"`
}

type CommitSummary struct {
	SHA         string    `json:"sha"`
	Subject     string    `json:"subject"`
	AuthorName  string    `json:"author_name"`
	AuthorEmail string    `json:"author_email"`
	CommittedAt string    `json:"committed_at"`
	Releases    []Release `json:"releases"`
	Sources     []string  `json:"sources"` // only on /messages/:b64id/commits
}

type CommitFile struct {
	Path       string `json:"path"`
	ChangeType string `json:"change_type"`
	Additions  int    `json:"additions"`
	Deletions  int    `json:"deletions"`
}

type Trailer struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type CommitFull struct {
	CommitSummary
	Body           string       `json:"body"`
	AuthoredAt     string       `json:"authored_at"`
	CommitterName  string       `json:"committer_name"`
	CommitterEmail string       `json:"committer_email"`
	Trailers       []Trailer    `json:"trailers"`
	Files          []CommitFile `json:"files"`
}

type CommitGroup struct {
	Subject string          `json:"subject"`
	Commits []CommitSummary `json:"commits"`
}

type CommitThread struct {
	Commit  CommitSummary `json:"commit"`
	Threads []struct {
		ThreadID string           `json:"thread_id"`
		Messages []MessageSummary `json:"messages"`
	} `json:"threads"`
	UnresolvedRefs []struct {
		Source       string `json:"source"`
		RefMessageID string `json:"ref_message_id"`
	} `json:"unresolved_refs"`
}

type SenderStats struct {
	MessageCount   int     `json:"message_count"`
	FirstMessageAt *string `json:"first_message_at"`
	LastMessageAt  *string `json:"last_message_at"`
}

type SenderSummary struct {
	ID          int          `json:"id"`
	Email       string       `json:"email"`
	DisplayName string       `json:"display_name"`
	Stats       *SenderStats `json:"stats"`
}

type MessagePreview struct {
	MessageID string `json:"message_id"`
	Subject   string `json:"subject"`
	SentAt    string `json:"sent_at"`
}

type SenderFull struct {
	SenderSummary
	Messages []MessagePreview `json:"messages"`
}

type ThreadStarter struct {
	MessageID string  `json:"message_id"`
	Subject   string  `json:"subject"`
	SentAt    *string `json:"sent_at"`
	Sender    *Sender `json:"sender"`
}

type ThreadSummary struct {
	ThreadID       string         `json:"thread_id"`
	Subject        string         `json:"subject"`
	MessageCount   int            `json:"message_count"`
	StartedAt      string         `json:"started_at"`
	LastActivityAt string         `json:"last_activity_at"`
	Starter        *ThreadStarter `json:"starter"`
}

type AttachmentRow struct {
	ID          int     `json:"id"`
	Filename    string  `json:"filename"`
	ContentType string  `json:"content_type"`
	Size        int     `json:"size"`
	MessageID   *string `json:"message_id"`
	IsPatch     bool    `json:"is_patch"`
}

type PatchFile struct {
	Path      string `json:"path"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

type Patch struct {
	Format        string      `json:"format"`
	SeriesVersion *int        `json:"series_version"`
	SeriesSeq     *int        `json:"series_seq"`
	Subject       string      `json:"subject"`
	Files         []PatchFile `json:"files"`
}

type ActivityRow struct {
	Kind       string `json:"kind"`
	ActivityAt string `json:"activity_at"`
	SHA        string `json:"sha"`
	MessageID  string `json:"message_id"`
	Subject    string `json:"subject"`
}

type MinorRelease struct {
	Tag        string  `json:"tag"`
	Minor      int     `json:"minor"`
	ReleasedOn *string `json:"released_on"`
}

type Major struct {
	Major      string         `json:"major"`
	DocSlug    string         `json:"doc_slug"`
	ReleasedOn *string        `json:"released_on"`
	EolOn      *string        `json:"eol_on"`
	Releases   []MinorRelease `json:"releases"`
}

type Guc struct {
	Name      string `json:"name"`
	Vartype   string `json:"vartype"`
	BootVal   string `json:"boot_val"`
	Context   string `json:"context"`
	Category  string `json:"category"`
	ShortDesc string `json:"short_desc"`
}

type GucCatalog struct {
	Major string `json:"major"`
	Gucs  []Guc  `json:"gucs"`
}

type GucChange struct {
	Name    string `json:"name"`
	Vartype string `json:"vartype"`
	From    string `json:"from"`
	To      string `json:"to"`
}

type GucDiff struct {
	Major          string      `json:"major"`
	ChangedSince   string      `json:"changed_since"`
	Added          []Guc       `json:"added"`
	Removed        []Guc       `json:"removed"`
	DefaultChanged []GucChange `json:"default_changed"`
	ContextChanged []GucChange `json:"context_changed"`
}

type DocPage struct {
	Title      *string `json:"title"`
	PageSlug   string  `json:"page_slug"`
	SgmlSource string  `json:"sgml_source"`
	URL        string  `json:"url"`
}

type DocPages struct {
	Major string    `json:"major"`
	Docs  []DocPage `json:"docs"`
}

type ImportFile struct {
	Basename   string `json:"basename"`
	Path       string `json:"path"`
	Size       int64  `json:"size"`
	SHA256     string `json:"sha256"`
	InsertedAt string `json:"inserted_at"`
}
