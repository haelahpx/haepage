// Command haepage runs the Haepage server.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // calendar time zones; the Docker image has no zoneinfo

	"github.com/haelahpx/haepage/internal/config"
	"github.com/haelahpx/haepage/internal/discovery"
	"github.com/haelahpx/haepage/internal/health"
	"github.com/haelahpx/haepage/internal/history"
	"github.com/haelahpx/haepage/internal/integrations"
	"github.com/haelahpx/haepage/internal/proxy"
	"github.com/haelahpx/haepage/internal/rdap"
	"github.com/haelahpx/haepage/internal/server"
	"github.com/haelahpx/haepage/internal/store"
	"github.com/haelahpx/haepage/internal/system"
	"github.com/haelahpx/haepage/internal/tailscale"
	"github.com/haelahpx/haepage/internal/themes"
	"github.com/haelahpx/haepage/internal/version"
	"github.com/haelahpx/haepage/internal/wallpaper"
	"github.com/haelahpx/haepage/web"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "haepage:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 0 && args[0] == "lint-theme" {
		return lintTheme(os.Stdout, args[1:])
	}
	flags := flag.NewFlagSet("haepage", flag.ContinueOnError)
	addr := flags.String("addr", envOr("HAEPAGE_ADDR", ":8080"), "listen address")
	dataDir := flags.String("data", envOr("HAEPAGE_DATA", "./data"), "data folder (config, secrets, cache)")
	configPath := flags.String("config", envOr("HAEPAGE_CONFIG", ""), "config file (default <data>/haepage.json)")
	noSystem := flags.Bool("no-system", false, "turn off the server stats tile and /api/system")
	dockerHost := flags.String("docker", envOr("DOCKER_HOST", "unix:///var/run/docker.sock"), "Docker socket or socket proxy (unix:// or tcp://)")
	noDocker := flags.Bool("no-docker", false, "turn off Docker discovery")
	noPorts := flags.Bool("no-ports", false, "turn off listening-port discovery")
	noHealth := flags.Bool("no-health", false, "turn off server-side health checks")
	tsSocket := flags.String("tailscale-socket", envOr("HAEPAGE_TAILSCALE_SOCKET", tailscale.DefaultSocket), "tailscaled socket, for the devices panel")
	noTailscale := flags.Bool("no-tailscale", false, "turn off the Tailscale devices panel")
	checkHost := flags.String("check-host", envOr("HAEPAGE_CHECK_HOST", "127.0.0.1"), "host that health checks use for services on this machine (host.docker.internal when Haepage runs in Docker)")
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

	if *configPath == "" {
		*configPath = filepath.Join(*dataDir, "haepage.json")
	}
	cfg, err := config.Open(*configPath)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	reg := store.NewRegistry()
	svc := &services{cfg: cfg, reg: reg, isLocal: discovery.LocalHosts(), checkHost: *checkHost}
	cfg.OnChange = svc.configChanged
	svc.remerge()
	go cfg.Watch(ctx, 2*time.Second, func(err error) { logger.Warn("config on disk is invalid; keeping the last good one", "err", err) })

	runner := &discovery.Runner{OnChange: svc.setDetected, Logger: logger, OwnPort: portOf(*addr)}
	if runner.Apps, err = discovery.LoadKnownApps(); err != nil {
		return err
	}
	if !*noDocker {
		if runner.Docker, err = dockerClient(*dockerHost); err != nil {
			logger.Info("docker discovery off", "reason", err)
		}
	}
	if !*noPorts {
		runner.Ports = discovery.NewPortScanner()
	}
	detect := runner.Docker != nil || runner.Ports != nil
	if detect {
		go runner.Run(ctx)
	}
	var hist *history.History
	if !*noHealth {
		// Every check result also goes into the 24-hour uptime history,
		// which is saved every 5 minutes and on shutdown.
		hist = history.Open(filepath.Join(*dataDir, "cache", "state.json"))
		sched := &health.Scheduler{Targets: svc.targets, Report: func(id string, h store.Health) {
			reg.SetHealth(id, h)
			hist.Record(id, h.Status)
		}}
		go sched.Run(ctx)
		go func() {
			tick := time.NewTicker(5 * time.Minute)
			defer tick.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-tick.C:
					if err := hist.Save(); err != nil {
						logger.Warn("saving uptime history", "err", err)
					}
				}
			}
		}()
	}

	secrets, err := config.OpenSecrets(filepath.Join(*dataDir, "secrets.json"))
	if err != nil {
		return err
	}
	integ := &integrations.Manager{Registry: reg, Secrets: secrets, CheckHost: *checkHost, Kinds: integrations.Kinds(&http.Client{})}
	go integ.Run(ctx)

	opts := server.Options{
		Version: version.Version, Static: static, Dev: *dev,
		Config: cfg, Registry: reg, Detect: detect, Health: !*noHealth, Integrations: integ,
	}
	if !*noSystem {
		opts.System = system.NewSampler("/", *dataDir)
	}
	guard := &proxy.Guard{AllowPrivate: func(host string) bool { return cfg.Config().AllowsPrivateHost(host) }}
	if !*noTailscale {
		if opts.Tailscale, err = tailscale.New(*tsSocket); err != nil {
			logger.Info("tailscale devices off", "reason", err)
		}
	}
	opts.Wallpapers = &wallpaper.Library{Dir: filepath.Join(*dataDir, "wallpapers"), Cache: filepath.Join(*dataDir, "cache", "wallpapers")}
	opts.Themes = &themes.Remote{Dir: filepath.Join(*dataDir, "cache", "themes"), Client: guard.Client(10 * time.Second)}
	opts.Secrets = secrets
	opts.History = hist
	opts.Widgets = &proxy.Widgets{
		Config:  cfg.Config,
		Secrets: secrets,
		Client:  guard.Client(10 * time.Second),
		RDAP:    &rdap.Client{HTTP: guard.Client(10 * time.Second)},
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           server.New(opts),
		ReadHeaderTimeout: 10 * time.Second,
		// No WriteTimeout: server-sent events keep responses open. Requests
		// share the root context, so open event streams end on shutdown.
		BaseContext: func(net.Listener) context.Context { return ctx },
	}

	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	logger.Info("listening", "addr", *addr, "version", version.Version, "config", *configPath, "docker", runner.Docker != nil, "ports", runner.Ports != nil, "dev", *dev)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	if hist != nil {
		if err := hist.Save(); err != nil {
			logger.Warn("saving uptime history", "err", err)
		}
	}
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

// dockerClient returns nil with a reason when the default socket is absent,
// so machines without Docker start quietly.
func dockerClient(host string) (*discovery.DockerClient, error) {
	if path, ok := strings.CutPrefix(host, "unix://"); ok {
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("no Docker socket at %s", path)
		}
	}
	return discovery.NewDockerClient(host)
}

// portOf returns the port of a listen address like ":8080".
func portOf(addr string) int {
	_, p, err := net.SplitHostPort(addr)
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(p)
	return n
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
