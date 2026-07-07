package config

import (
	"os"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// Missing file: zero config, no error.
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load with no file: %v", err)
	}
	if cfg != (Config{}) {
		t.Errorf("got %+v, want zero", cfg)
	}

	saved := Config{Host: "http://localhost:4000", APIKey: "hml_test"}
	if err := Save(saved); err != nil {
		t.Fatal(err)
	}

	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Errorf("mode = %o, want 600", st.Mode().Perm())
	}

	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg != saved {
		t.Errorf("got %+v, want %+v", cfg, saved)
	}

	if err := Remove(); err != nil {
		t.Fatal(err)
	}
	if err := Remove(); err != nil {
		t.Errorf("second Remove should be a no-op, got %v", err)
	}
}
