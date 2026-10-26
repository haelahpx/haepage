// Command haepage runs the Haepage server.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/haelahpx/haepage/internal/server"
	"github.com/haelahpx/haepage/internal/version"
	"github.com/haelahpx/haepage/web"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "haepage:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("haepage", flag.ContinueOnError)
	addr := flags.String("addr", envOr("HAEPAGE_ADDR", ":8080"), "listen address")
	logFormat := flags.String("log-format", envOr("HAEPAGE_LOG_FORMAT", "text"), "log format: text or json")
	logLevel := flags.String("log-level", envOr("HAEPAGE_LOG_LEVEL", "info"), "log level: debug, info, warn or error")
	dev := flags.Bool("dev", false, "serve web/dist from disk (run from the repo root) and turn off caching")
	showVersion := flags.Bool("version", false, "print the version and exit")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	if *showVersion {
		fmt.Println(version.String())
		return nil
	}

	logger, err := newLogger(*logFormat, *logLevel)
	if err != nil {
		return err
	}
	slog.SetDefault(logger)

	// Keep the heap small by default; GOMEMLIMIT in the environment wins.
	if os.Getenv("GOMEMLIMIT") == "" {
		debug.SetMemoryLimit(64 << 20)
	}

	var static fs.FS = web.Dist()
	if *dev {
		static = os.DirFS("web/dist")
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           server.New(server.Options{Version: version.Version, Static: static, Dev: *dev}),
		ReadHeaderTimeout: 10 * time.Second,
		// No WriteTimeout: server-sent events keep responses open.
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	logger.Info("listening", "addr", *addr, "version", version.Version, "dev", *dev)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func newLogger(format, level string) (*slog.Logger, error) {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("log level: %w", err)
	}
	opts := &slog.HandlerOptions{Level: lvl}
	switch format {
	case "text":
		return slog.New(slog.NewTextHandler(os.Stderr, opts)), nil
	case "json":
		return slog.New(slog.NewJSONHandler(os.Stderr, opts)), nil
	}
	return nil, fmt.Errorf("unknown log format %q (want text or json)", format)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
