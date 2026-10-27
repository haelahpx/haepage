package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLintTheme(t *testing.T) {
	dir := t.TempDir()
	clean := filepath.Join(dir, "clean.css")
	slow := filepath.Join(dir, "slow.css")
	bad := filepath.Join(dir, "bad.css")
	os.WriteFile(clean, []byte(":root { --accent: red }"), 0o644)
	os.WriteFile(slow, []byte(".hp-card {\n  transition: width 1s;\n}"), 0o644)
	os.WriteFile(bad, []byte("@import 'x.css';"), 0o644)

	var out bytes.Buffer
	if err := lintTheme(&out, []string{clean, slow}); err != nil {
		t.Fatalf("warnings alone must pass: %v", err)
	}
	if !strings.Contains(out.String(), clean+": ok") || !strings.Contains(out.String(), slow+": line 2: warning:") {
		t.Errorf("output:\n%s", out.String())
	}
	if err := lintTheme(&out, []string{"-strict", slow}); err == nil {
		t.Error("-strict must fail on warnings")
	}
	if err := lintTheme(&out, []string{bad}); err == nil {
		t.Error("errors must fail")
	}
	if err := lintTheme(&out, []string{filepath.Join(dir, "missing.css")}); err == nil {
		t.Error("a missing file must fail")
	}
	if err := lintTheme(&out, nil); err == nil {
		t.Error("no arguments must fail")
	}
}
