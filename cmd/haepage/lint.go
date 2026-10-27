package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/haelahpx/haepage/internal/themes"
)

// lintTheme runs `haepage lint-theme [-strict] <file or url>...`. It fails
// on errors, and on warnings too with -strict.
func lintTheme(w io.Writer, args []string) error {
	flags := flag.NewFlagSet("lint-theme", flag.ContinueOnError)
	strict := flags.Bool("strict", false, "fail on warnings too")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "Usage: haepage lint-theme [-strict] <file or https:// url>...")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() == 0 {
		flags.Usage()
		return errors.New("lint-theme needs a file or a link")
	}

	failed := 0
	client := &http.Client{Timeout: 10 * time.Second}
	for _, target := range flags.Args() {
		css, err := readTheme(client, target)
		if err != nil {
			fmt.Fprintf(w, "%s: %v\n", target, err)
			failed++
			continue
		}
		findings := themes.Lint(css)
		if len(findings) == 0 {
			fmt.Fprintf(w, "%s: ok\n", target)
			continue
		}
		for _, f := range findings {
			fmt.Fprintf(w, "%s: %s\n", target, f)
		}
		if themes.HasErrors(findings) || *strict {
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d themes failed the lint", failed, flags.NArg())
	}
	return nil
}

func readTheme(client *http.Client, target string) ([]byte, error) {
	if strings.HasPrefix(target, "https://") || strings.HasPrefix(target, "http://") {
		return themes.Fetch(context.Background(), client, target)
	}
	return os.ReadFile(target)
}
