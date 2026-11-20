package cli

import (
	"net/url"
	"reflect"
	"testing"

	"git.kehvyn.dev/kevin/pglantern-cli/internal/api"
)

func TestThreadStarterCell(t *testing.T) {
	tests := []struct {
		name    string
		starter *api.ThreadStarter
		want    string
	}{
		{
			name:    "starter renders the display name, not the address",
			starter: &api.ThreadStarter{Sender: &api.Sender{Email: "tgl@sss.pgh.pa.us", DisplayName: "Tom Lane"}},
			want:    "Tom Lane",
		},
		{
			name:    "starter with no display name degrades to email",
			starter: &api.ThreadStarter{Sender: &api.Sender{Email: "tgl@sss.pgh.pa.us"}},
			want:    "tgl@sss.pgh.pa.us",
		},
		{
			name:    "thread with no ingested start renders a dash",
			starter: nil,
			want:    "-",
		},
		{
			name:    "starter with no sender renders a dash",
			starter: &api.ThreadStarter{MessageID: "28432.892671936@sss.pgh.pa.us"},
			want:    "-",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := threadStarterCell(tt.starter); got != tt.want {
				t.Errorf("threadStarterCell() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestThreadsFilterFlags(t *testing.T) {
	_, query, _ := runStub(t, `{"data":[]}`, "threads",
		"--min-messages", "51", "--max-patches", "3", "--min-participants", "11",
		"--first-from", "2025-01-01", "--first-to", "2025-06-30",
		"--list", "pgsql-hackers", "--list", "pgsql-bugs", "--exclude-list", "pgsql-general",
		"--starter-id", "48", "--last-sender-id", "7", "--exclude-starter-id", "9",
		"--committed=false", "--path", "src/backend/", "--major", "18")
	got, err := url.ParseQuery(query)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"min_messages":       {"51"},
		"max_patches":        {"3"},
		"min_participants":   {"11"},
		"first_from":         {"2025-01-01"},
		"first_to":           {"2025-06-30"},
		"list[]":             {"pgsql-hackers", "pgsql-bugs"},
		"exclude_list":       {"pgsql-general"},
		"starter_id":         {"48"},
		"last_sender_id":     {"7"},
		"exclude_starter_id": {"9"},
		"committed":          {"false"},
		"path":               {"src/backend/"},
		"major":              {"18"},
	}
	for k, v := range want {
		if !reflect.DeepEqual(got[k], v) {
			t.Errorf("%s = %v, want %v (query %q)", k, got[k], v, query)
		}
	}
	if _, ok := got["list"]; ok {
		t.Errorf("bare list= sent alongside list[]: %q", query)
	}
}

func TestThreadsUnsetFiltersSendNothing(t *testing.T) {
	_, query, _ := runStub(t, `{"data":[]}`, "threads", "--limit", "5")
	if query != "limit=5" {
		t.Errorf("query = %q", query)
	}
}
