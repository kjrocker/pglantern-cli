package cli

import "testing"

func TestNormalizeMessageID(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "raw message-id is encoded",
			in:   "2294297.1780270682@sss.pgh.pa.us",
			want: "MjI5NDI5Ny4xNzgwMjcwNjgyQHNzcy5wZ2gucGEudXM",
		},
		{
			name: "angle brackets from a mail header are stripped first",
			in:   "<2294297.1780270682@sss.pgh.pa.us>",
			want: "MjI5NDI5Ny4xNzgwMjcwNjgyQHNzcy5wZ2gucGEudXM",
		},
		{
			name: "already-encoded token passes through",
			in:   "MjI5NDI5Ny4xNzgwMjcwNjgyQHNzcy5wZ2gucGEudXM",
			want: "MjI5NDI5Ny4xNzgwMjcwNjgyQHNzcy5wZ2gucGEudXM",
		},
		{
			name: "gmail-style id with = and + is encoded, not mistaken for b64",
			in:   "CAFiTN-sF_J8NB3xjie7g=2-R5v9aLqEE5jrtF2dMmwPngd9RBg@mail.gmail.com",
			want: "Q0FGaVROLXNGX0o4TkIzeGppZTdnPTItUjV2OWFMcUVFNWpydEYyZE1td1BuZ2Q5UkJnQG1haWwuZ21haWwuY29t",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeMessageID(tt.in); got != tt.want {
				t.Errorf("normalizeMessageID(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
