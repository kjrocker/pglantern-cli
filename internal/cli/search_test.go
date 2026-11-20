package cli

import (
	"net/url"
	"reflect"
	"testing"
)

func TestSearchThreadFilterFlags(t *testing.T) {
	_, query, _ := runStub(t, `{"data":[]}`, "search", "vacuum", "truncate",
		"--list", "pgsql-hackers", "--list", "pgsql-bugs",
		"--min-patches", "1", "--starter-id", "48", "--starter-id", "7", "--first-to", "2025-12-31")
	got, err := url.ParseQuery(query)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"q":            {"vacuum truncate"},
		"list[]":       {"pgsql-hackers", "pgsql-bugs"},
		"min_patches":  {"1"},
		"starter_id[]": {"48", "7"},
		"first_to":     {"2025-12-31"},
	}
	for k, v := range want {
		if !reflect.DeepEqual(got[k], v) {
			t.Errorf("%s = %v, want %v (query %q)", k, got[k], v, query)
		}
	}
}

// One value stays a plain key=v (the shape servers before the list form took).
func TestSearchSingleListStaysPlain(t *testing.T) {
	_, query, _ := runStub(t, `{"data":[]}`, "search", "vacuum", "--list", "pgsql-hackers")
	got, _ := url.ParseQuery(query)
	if !reflect.DeepEqual(got["list"], []string{"pgsql-hackers"}) || got["list[]"] != nil {
		t.Errorf("query = %q", query)
	}
}

func TestSearchMessageGroupFlags(t *testing.T) {
	_, query, _ := runStub(t, `{"data":[]}`, "search",
		"--q-body", "deadlock", "--q-subject", "lock", "--sender-id", "48", "--sender-id", "7",
		"--has-patch", "--has-attachment=false", "--attachment-name", "v2",
		"--participant-id", "9", "--exclude-participant-id", "3", "--exclude-participant-id", "4")
	got, err := url.ParseQuery(query)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"q_body":                   {"deadlock"},
		"q_subject":                {"lock"},
		"sender_id[]":              {"48", "7"},
		"has_patch":                {"true"},
		"has_attachment":           {"false"},
		"attachment_name":          {"v2"},
		"participant_id":           {"9"},
		"exclude_participant_id[]": {"3", "4"},
	}
	for k, v := range want {
		if !reflect.DeepEqual(got[k], v) {
			t.Errorf("%s = %v, want %v (query %q)", k, got[k], v, query)
		}
	}
	if _, ok := got["q"]; ok {
		t.Errorf("q sent without a positional query: %q", query)
	}
}
