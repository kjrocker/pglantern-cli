package cli

import (
	"strings"
	"testing"
)

func TestAttachmentsRaw(t *testing.T) {
	patch := "From 0123 Mon Sep 17 00:00:00 2001\ndiff --git a/x b/x\n+caf\xe9"
	path, _, out := runStub(t, patch, "attachments", "raw", "1234")
	if !strings.HasSuffix(path, "/attachments/1234/raw") {
		t.Errorf("path = %q", path)
	}
	// Byte-exact: no decoding, no added trailing newline.
	if out != patch {
		t.Errorf("out = %q, want %q", out, patch)
	}
}

func TestAttachmentsRawRejectsNonIntegerID(t *testing.T) {
	root := NewRootCmd()
	root.SetArgs([]string{"attachments", "raw", "abc", "--host", "http://127.0.0.1:1", "--api-key", "k"})
	if err := root.Execute(); err == nil {
		t.Error("non-integer id accepted")
	}
}
