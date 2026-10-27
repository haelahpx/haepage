package themes

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func rules(findings []Finding) string {
	var parts []string
	for _, f := range findings {
		parts = append(parts, f.Rule+"@"+strconv.Itoa(f.Line))
	}
	return strings.Join(parts, " ")
}

func TestLintFindsEachRule(t *testing.T) {
	cases := []struct {
		name, css, want string
	}{
		{"clean", ":root { --accent: red; }\n.hp-card { transition: transform 200ms, opacity 200ms; }", ""},
		{"import", "@import url(https://x.example/a.css);\n:root{}", "import@1"},
		{"transition width", ":root{}\n.hp-card {\n  transition: width 200ms ease, transform 1s;\n}", "animation@3"},
		{"transition with calc", ".hp-card { transition: transform calc(320ms * var(--motion)) var(--ease), border-radius 1s; }", ""},
		{"transition all", ".hp-card { transition: all .2s }", "animation@1"},
		{"transition-property", ".hp-card { transition-property: opacity, filter }", "animation@1"},
		{"keyframes", "@keyframes grow {\n  from { height: 0 }\n  to { height: 10px; opacity: 1 }\n}", "animation@2 animation@3"},
		{"keyframes transform ok", "@keyframes spin { to { transform: rotate(1turn) } }", ""},
		{"blur on cards", ".hp-bento .hp-card:hover { backdrop-filter: blur(10px) }", "backdrop-filter@1"},
		{"blur on overlay ok", ".hp-drawer, .hp-toast { backdrop-filter: blur(20px) }", ""},
		{"large blur", ".hp-drawer { backdrop-filter: blur(60px) }\n:root { --blur: 48px }", "blur@1 blur@2"},
		{"many shadows", ".hp-tile { box-shadow: 0 0 1px a, 0 0 2px b, 0 0 3px c, 0 0 4px d, 0 0 5px e }", "shadow@1"},
		{"external url", ".hp-app { background: url('https://img.example/bg.jpg') }\n.x { background: url(data:image/png;base64,AAAA) }", "external@1"},
		{"read-only", ":root { --wallpaper: url(x.jpg); --motion: 2 }", "read-only@1 read-only@1"},
		{"font-face", "@font-face {\n  font-family: X;\n  src: url(x.woff2);\n}", "font-display@1"},
		{"font-face ok", "@font-face { font-family: X; src: url(x.woff2); font-display: swap }", ""},
		{"will-change", ".hp-card { will-change: transform, height }", "animation@1"},
		{"comments and strings", "/* transition: width 1s; @import x;\n */\n.hp-card::after { content: \"a;b{c}\"; transition: height 1s }", "animation@3"},
		{"media nesting", "@media (max-width: 600px) {\n  .hp-card { transition: left 1s }\n}", "animation@2"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := rules(Lint([]byte(c.css))); got != c.want {
				t.Errorf("got %q, want %q\n%v", got, c.want, Lint([]byte(c.css)))
			}
		})
	}
}

func TestLintSeverity(t *testing.T) {
	if !HasErrors(Lint([]byte("@import 'x.css';"))) {
		t.Error("@import must be an error")
	}
	if HasErrors(Lint([]byte(".hp-card{transition:width 1s}"))) {
		t.Error("budget findings must be warnings")
	}
	big := []byte(":root{--a:" + strings.Repeat("x", BudgetSize) + "}")
	if f := Lint(big); len(f) != 1 || f[0].Rule != "size" || f[0].Severity != SeverityWarning {
		t.Errorf("over budget: %v", f)
	}
	huge := []byte(":root{--a:" + strings.Repeat("x", MaxSize) + "}")
	if !HasErrors(Lint(huge)) {
		t.Error("over the import cap must be an error")
	}
}

func TestLintManyBlurSelectors(t *testing.T) {
	css := ".hp-drawer,.hp-dialog,.hp-toast,.hp-palette,.hp-search-results,.hp-banner,.hp-header { backdrop-filter: blur(8px) }"
	if got := rules(Lint([]byte(css))); got != "backdrop-filter@0" {
		t.Errorf("got %q", got)
	}
}

// The built-in themes must pass with no findings at all.
func TestBuiltinThemesAreClean(t *testing.T) {
	files, _ := filepath.Glob("../../themes/*/style.css")
	if len(files) < 2 {
		t.Fatalf("found %d built-in themes", len(files))
	}
	for _, f := range files {
		css, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if findings := Lint(css); len(findings) > 0 {
			t.Errorf("%s:\n%v", f, findings)
		}
	}
}
