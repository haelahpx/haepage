package themes

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Budgets from IMPLEMENTATION.md §5.4 and docs/theme-api.md.
const (
	BudgetSize       = 50 << 10 // bytes
	maxBlur          = 40       // px
	maxShadowLayers  = 4
	maxBlurSelectors = 6 // overlays only appear one or two at a time
)

// Severity of a lint finding. Errors are refused on import; warnings are
// budget or performance advice.
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
)

// Finding is one lint result. Line is 1-based, or 0 for the whole file.
type Finding struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Line     int    `json:"line"`
	Message  string `json:"message"`
}

func (f Finding) String() string {
	if f.Line == 0 {
		return fmt.Sprintf("%s: %s [%s]", f.Severity, f.Message, f.Rule)
	}
	return fmt.Sprintf("line %d: %s: %s [%s]", f.Line, f.Severity, f.Message, f.Rule)
}

// HasErrors reports whether any finding is an error.
func HasErrors(findings []Finding) bool {
	for _, f := range findings {
		if f.Severity == SeverityError {
			return true
		}
	}
	return false
}

// decl is one property: value pair with where it sits.
type decl struct {
	prop, value string
	line        int
	selectors   string // the enclosing rule's prelude
	keyframes   bool   // inside @keyframes
}

// atRule is a statement such as @import, or the prelude of a block at-rule.
type atRule struct {
	text  string
	line  int
	decls []decl // for blocks such as @font-face
}

// parse is a forgiving CSS reader: enough to find declarations, their
// selectors and at-rules. It never fails; broken CSS just yields less.
func parse(css string) (decls []decl, ats []*atRule) {
	type frame struct {
		prelude   string
		keyframes bool
		at        *atRule
	}
	var (
		stack     []frame
		buf       strings.Builder
		line      = 1
		startLine = 0
		quote     byte
		parens    int
	)
	flush := func() (string, int) {
		text := strings.TrimSpace(buf.String())
		buf.Reset()
		l := startLine
		startLine = 0
		return text, l
	}
	emit := func(text string, l int) {
		if text == "" {
			return
		}
		if strings.HasPrefix(text, "@") {
			ats = append(ats, &atRule{text: text, line: l})
			return
		}
		prop, value, ok := strings.Cut(text, ":")
		if !ok || len(stack) == 0 {
			return
		}
		top := stack[len(stack)-1]
		d := decl{
			prop:      strings.ToLower(strings.TrimSpace(prop)),
			value:     strings.TrimSpace(value),
			line:      l,
			selectors: top.prelude,
			keyframes: top.keyframes,
		}
		decls = append(decls, d)
		if top.at != nil {
			top.at.decls = append(top.at.decls, d)
		}
	}

	for i := 0; i < len(css); i++ {
		c := css[i]
		if c == '\n' {
			line++
		}
		switch {
		case quote != 0:
			buf.WriteByte(c)
			if c == '\\' && i+1 < len(css) {
				i++
				buf.WriteByte(css[i])
			} else if c == quote {
				quote = 0
			}
			continue
		case c == '/' && i+1 < len(css) && css[i+1] == '*':
			end := strings.Index(css[i+2:], "*/")
			if end < 0 {
				end = len(css) - i - 2
			}
			line += strings.Count(css[i:i+2+end], "\n")
			i += end + 3
			buf.WriteByte(' ')
			continue
		case c == '"' || c == '\'':
			quote = c
		case c == '(':
			parens++
		case c == ')' && parens > 0:
			parens--
		case parens > 0:
		case c == '{':
			text, l := flush()
			f := frame{prelude: text}
			lower := strings.ToLower(text)
			if len(stack) > 0 && stack[len(stack)-1].keyframes {
				f.keyframes = true
			}
			if strings.HasPrefix(lower, "@") {
				f.at = &atRule{text: text, line: l}
				ats = append(ats, f.at)
				if strings.Contains(lower, "keyframes") {
					f.keyframes = true
				}
			}
			stack = append(stack, f)
			continue
		case c == ';':
			emit(flush())
			continue
		case c == '}':
			emit(flush())
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			continue
		}
		if startLine == 0 && c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			startLine = line
		}
		buf.WriteByte(c)
	}
	emit(flush())
	return decls, ats
}

// splitTop splits s on sep where it isn't inside parentheses or quotes.
func splitTop(s string, sep byte) []string {
	var parts []string
	depth, start := 0, 0
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '(':
			depth++
		case c == ')':
			depth--
		case c == sep && depth == 0:
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return append(parts, s[start:])
}

// Properties that make the browser lay out or repaint on every frame.
var layoutProps = map[string]bool{
	"width": true, "height": true, "top": true, "left": true, "right": true, "bottom": true,
	"inset": true, "margin": true, "padding": true, "filter": true,
	"backdrop-filter": true, "-webkit-backdrop-filter": true,
	"min-width": true, "max-width": true, "min-height": true, "max-height": true,
	"font-size": true, "border-width": true,
}

// Elements that repeat on a page; a live blur on each one is expensive.
var repeatedClass = regexp.MustCompile(`\.hp-(card|tile|group|result|widget|media|chip|badge|status|seg|metric|activity|avatar|bar)\b(?:[^-]|$)`)

var (
	blurRadius  = regexp.MustCompile(`blur\(\s*([0-9.]+)px`)
	externalURL = regexp.MustCompile(`(?i)url\(\s*['"]?\s*((?:https?:)?//[^'")\s]+)`)
	readOnly    = map[string]bool{"--wallpaper": true, "--wallpaper-blurred": true, "--press-x": true, "--press-y": true, "--motion": true}
)

// Lint checks theme CSS against the theme budgets. Findings are in file
// order, with whole-file findings first.
func Lint(css []byte) []Finding {
	var out []Finding
	add := func(rule, sev string, line int, format string, args ...any) {
		out = append(out, Finding{Rule: rule, Severity: sev, Line: line, Message: fmt.Sprintf(format, args...)})
	}

	switch {
	case len(css) > MaxSize:
		add("size", SeverityError, 0, "the file is %d KB; themes over %d KB are refused", len(css)>>10, MaxSize>>10)
	case len(css) > BudgetSize:
		add("size", SeverityWarning, 0, "the file is %d KB; keep themes under %d KB", len(css)>>10, BudgetSize>>10)
	}

	decls, ats := parse(string(css))

	for _, a := range ats {
		lower := strings.ToLower(a.text)
		switch {
		case strings.HasPrefix(lower, "@import"):
			add("import", SeverityError, a.line, "@import is not allowed: the browser would fetch it from another site")
		case strings.HasPrefix(lower, "@font-face"):
			hasDisplay := false
			for _, d := range a.decls {
				hasDisplay = hasDisplay || d.prop == "font-display"
			}
			if !hasDisplay {
				add("font-display", SeverityWarning, a.line, "@font-face without font-display: swap hides text until the font loads")
			}
		}
	}

	blurSelectors := map[string]int{}
	for _, d := range decls {
		value := strings.ToLower(d.value)
		switch {
		case d.keyframes && layoutProps[d.prop]:
			add("animation", SeverityWarning, d.line, "@keyframes animates %s; animate only transform and opacity", d.prop)

		case d.prop == "transition" || d.prop == "transition-property":
			for _, part := range splitTop(value, ',') {
				for _, tok := range strings.Fields(part) {
					if tok == "all" {
						add("animation", SeverityWarning, d.line, "transition: all also animates layout properties; list transform and opacity instead")
					} else if layoutProps[tok] {
						add("animation", SeverityWarning, d.line, "transition on %s; animate only transform and opacity", tok)
					}
				}
			}

		case d.prop == "will-change":
			for _, tok := range splitTop(value, ',') {
				if tok = strings.TrimSpace(tok); layoutProps[tok] {
					add("animation", SeverityWarning, d.line, "will-change: %s promises a layout animation; use transform or opacity", tok)
				}
			}

		case readOnly[d.prop]:
			add("read-only", SeverityWarning, d.line, "%s is set by Haepage; themes should read it, not set it", d.prop)

		case d.prop == "box-shadow" && value != "none":
			if n := len(splitTop(value, ',')); n > maxShadowLayers {
				add("shadow", SeverityWarning, d.line, "box-shadow has %d layers; keep to %d", n, maxShadowLayers)
			}
		}

		if (d.prop == "backdrop-filter" || d.prop == "-webkit-backdrop-filter") && value != "none" {
			for _, sel := range splitTop(d.selectors, ',') {
				sel = strings.TrimSpace(sel)
				if _, seen := blurSelectors[sel]; !seen {
					blurSelectors[sel] = d.line
					if subject := lastCompound(sel); repeatedClass.MatchString(subject) {
						add("backdrop-filter", SeverityWarning, d.line,
							"live backdrop-filter on %s, which repeats on the page; use a solid or --wallpaper-blurred background instead", subject)
					}
				}
			}
		}

		for _, m := range blurRadius.FindAllStringSubmatch(value, -1) {
			if px, _ := strconv.ParseFloat(m[1], 64); px > maxBlur {
				add("blur", SeverityWarning, d.line, "blur(%spx) is large; keep blur at %dpx or less", m[1], maxBlur)
			}
		}
		if d.prop == "--blur" {
			if px, err := strconv.ParseFloat(strings.TrimSuffix(value, "px"), 64); err == nil && px > maxBlur {
				add("blur", SeverityWarning, d.line, "--blur: %s is large; keep blur at %dpx or less", d.value, maxBlur)
			}
		}

		for _, m := range externalURL.FindAllStringSubmatch(d.value, -1) {
			add("external", SeverityWarning, d.line, "loads %s from another site: every visitor's browser contacts it", m[1])
		}
	}

	if len(blurSelectors) > maxBlurSelectors {
		add("backdrop-filter", SeverityWarning, 0, "live backdrop-filter on %d selectors; keep to %d", len(blurSelectors), maxBlurSelectors)
	}
	return sortFindings(out)
}

// lastCompound returns the part of a selector that picks the element
// (".a .b:hover" → ".b:hover").
func lastCompound(sel string) string {
	fields := strings.FieldsFunc(sel, func(r rune) bool { return r == ' ' || r == '>' || r == '+' || r == '~' })
	if len(fields) == 0 {
		return sel
	}
	return fields[len(fields)-1]
}

func sortFindings(f []Finding) []Finding {
	// Insertion sort: lists are short and this keeps equal lines in order.
	for i := 1; i < len(f); i++ {
		for j := i; j > 0 && f[j].Line < f[j-1].Line; j-- {
			f[j], f[j-1] = f[j-1], f[j]
		}
	}
	return f
}
