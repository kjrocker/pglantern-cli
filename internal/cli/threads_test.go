package cli

import (
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
