package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/user/extorvm/controller/gui"
	"github.com/user/extorvm/controller/internal/config"
	"github.com/user/extorvm/controller/internal/launchd"
	"github.com/user/extorvm/controller/internal/lifecycle"
	"github.com/user/extorvm/controller/internal/logging"
	"github.com/user/extorvm/controller/internal/metrics"
	"github.com/user/extorvm/controller/internal/platform"
	"github.com/user/extorvm/controller/internal/systemd"
	"github.com/user/extorvm/controller/internal/tor"
	"github.com/user/extorvm/controller/internal/winsvc"
)

func main() {
	var (
		accelFlag        = flag.String("accel", "", "acceleration backend: kvm, hvf, whpx, tcg")
		verboseFlag      = flag.Bool("verbose", false, "enable debug logging")
		headless         = flag.Bool("headless", false, "run without GUI")
		configFile       = flag.String("config", "", "path to JSON config file")
		clean            = flag.Bool("clean", false, "remove state disk before starting")
		replace          = flag.Bool("replace", false, "replace existing state disk with fresh copy")
		serviceInstall   = flag.Bool("service-install", false, "install as system service and exit")
		serviceUninstall = flag.Bool("service-uninstall", false, "uninstall system service and exit")
		serviceRun       = flag.Bool("service-run", false, "run as Windows service (used by SCM, not for manual invocation)")
		metricsAddr      = flag.String("metrics-addr", "", "address for metrics/health HTTP server (e.g. 127.0.0.1:9100)")
		logFormat        = flag.String("log-format", "", "log format: text (default) or json")
		logFile          = flag.String("log-file", "", "path to log file (in addition to stderr)")
		timeout          = flag.Duration("timeout", 0, "maximum runtime duration; 0 means unlimited")
		status           = flag.Bool("status", false, "query running instance status and exit")
		version          = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	if *version {
		fmt.Println("torvm version 0.1.0")
		return
	}

	// Handle service install/uninstall commands and exit.
	if *serviceInstall {
		var err error
		switch runtime.GOOS {
		case "darwin":
			err = launchd.Install(false)
		case "linux":
			err = systemd.Install()
		case "windows":
			err = winsvc.InstallService()
		default:
			fmt.Fprintf(os.Stderr, "error: service install not supported on %s\n", runtime.GOOS)
			os.Exit(1)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: service install: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("TorVM service installed.")
		return
	}
	if *serviceUninstall {
		var err error
		switch runtime.GOOS {
		case "darwin":
			err = launchd.Uninstall()
		case "linux":
			err = systemd.Uninstall()
		case "windows":
			err = winsvc.RemoveService()
		default:
			fmt.Fprintf(os.Stderr, "error: service uninstall not supported on %s\n", runtime.GOOS)
			os.Exit(1)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: service uninstall: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("TorVM service uninstalled.")
		return
	}

	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: load config: %v\n", err)
		os.Exit(1)
	}

	// Handle --status: query running instance and exit.
	if *status {
		exitCode := queryStatus(cfg)
		os.Exit(exitCode)
	}

	cfg.Verbose = *verboseFlag

	// Detect platform capabilities.
	platInfo, _ := platform.Detect()

	if *accelFlag != "" {
		accel, err := platform.ParseAccel(*accelFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		cfg.Accel = string(accel)
	} else {
		cfg.Accel = string(platInfo.Accel)
	}

	// Propagate runtime-detected capabilities to config.
	cfg.VhostNet = platInfo.VhostNet
	cfg.IOMMUEnabled = platInfo.IOMMUSupport

	logger, err := logging.NewLogger(logging.Options{
		Verbose: cfg.Verbose,
		LogFile: *logFile,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: create logger: %v\n", err)
		os.Exit(1)
	}

	// If JSON log format requested, add a JSON writer to the logger.
	if *logFormat == "json" {
		jw := logging.NewJSONWriter(os.Stderr)
		logger.AddWriter(jw)
	}

	logger.Info("TorVM controller starting (accel=%s)", cfg.Accel)

	// If running as a Windows service, hand off to the SCM handler.
	if *serviceRun {
		if err := winsvc.RunService(cfg, logger); err != nil {
			logger.Error("windows service error: %v", err)
			os.Exit(1)
		}
		return
	}

	// Set up Prometheus metrics and optional HTTP server.
	var recorder *metrics.Recorder
	reg := prometheus.NewRegistry()
	recorder = metrics.NewRecorder(reg)
	defer recorder.Stop()

	// engineRef is set once the engine is created (below), so the health
	// endpoint can report live state.
	var engineRef *lifecycle.Engine

	startTime := time.Now()
	var lastError string

	if *metricsAddr != "" {
		healthFn := func() metrics.HealthStatus {
			if engineRef == nil {
				return metrics.HealthStatus{
					State:   "Init",
					Version: "0.1.0",
				}
			}
			bootstrap := 0
			if engineRef.State() == lifecycle.StateRunning {
				bootstrap = 100
			}
			return metrics.HealthStatus{
				State:            engineRef.State().String(),
				Bootstrap:        bootstrap,
				Failsafe:         engineRef.FailSafe.IsActive(),
				UptimeSeconds:    int(time.Since(startTime).Seconds()),
				BootstrapPercent: bootstrap,
				LastError:        lastError,
				Version:          "0.1.0",
			}
		}
		metricsSrv, mErr := metrics.NewServer(*metricsAddr, reg, healthFn)
		if mErr != nil {
			fmt.Fprintf(os.Stderr, "error: metrics server: %v\n", mErr)
			os.Exit(1)
		}
		metricsSrv.Start()
		logger.Info("metrics server listening on %s", metricsSrv.Addr())
		defer metricsSrv.Shutdown(context.Background())
	}

	// Handle --clean: remove state disk.
	if *clean || *replace {
		logger.Info("removing state disk: %s", cfg.StateDiskPath)
		os.Remove(cfg.StateDiskPath)
	}

	if *headless {
		// CLI mode: blocking lifecycle with optional systemd integration.
		var ctx context.Context
		var cancel context.CancelFunc
		if *timeout > 0 {
			ctx, cancel = context.WithTimeout(context.Background(), *timeout)
			logger.Info("maximum runtime set to %v", *timeout)
		} else {
			ctx, cancel = context.WithCancel(context.Background())
		}
		defer cancel()

		// If running under systemd, attach journal writer and set up notifications.
		underSystemd := systemd.IsRunningUnderSystemd()
		if underSystemd {
			jw, jwErr := systemd.NewJournalWriter()
			if jwErr != nil {
				logger.Error("failed to open journal writer: %v", jwErr)
			} else {
				logger.AddWriter(jw)
				defer jw.Close()
			}
			_ = systemd.Status("starting")
		}

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			sig := <-sigCh
			logger.Info("received signal %v, shutting down", sig)
			if underSystemd {
				_ = systemd.Stopping()
			}
			cancel()
		}()

		engine := lifecycle.NewEngine(cfg, logger)
		engine.Metrics = recorder
		engineRef = engine

		// Start config file watcher for hot reload.
		if *configFile != "" {
			watcher, wErr := config.NewConfigWatcher(*configFile, func(newCfg *config.Config) {
				diff := config.Diff(engine.Config, newCfg)
				if !diff.HasChanges() {
					return
				}
				for _, field := range diff.RestartRequired {
					logger.Info("config watcher: %s changed, restart required", field)
				}
				if len(diff.HotReloadable) > 0 {
					if rErr := engine.ReloadConfig(newCfg); rErr != nil {
						logger.Error("config watcher: reload failed: %v", rErr)
					}
				}
			})
			if wErr != nil {
				logger.Error("config watcher: %v", wErr)
			} else {
				defer watcher.Close()
			}
		}

		// Register systemd state observer for Ready/Status notifications.
		if underSystemd {
			watchdogStop := make(chan struct{})
			engine.OnStateChange(func(from, to lifecycle.State) {
				switch to {
				case lifecycle.StateRunning:
					_ = systemd.Ready()
					_ = systemd.Status("running - Tor connected")
					// Start watchdog goroutine (WatchdogSec=60, ping every 25s).
					go func() {
						ticker := time.NewTicker(25 * time.Second)
						defer ticker.Stop()
						for {
							select {
							case <-ticker.C:
								_ = systemd.Watchdog()
							case <-watchdogStop:
								return
							}
						}
					}()
				case lifecycle.StateShutdown:
					close(watchdogStop)
					_ = systemd.Status("shutting down")
				case lifecycle.StateWaitBootstrap:
					_ = systemd.Status("waiting for Tor bootstrap")
				case lifecycle.StateLaunchVM:
					_ = systemd.Status("launching VM")
				case lifecycle.StateCreateTAP:
					_ = systemd.Status("creating TAP adapter")
				}
			})
		}

		if err := engine.Run(ctx); err != nil {
			lastError = err.Error()
			logger.Error("lifecycle error: %v", err)
			os.Exit(1)
		}

		logger.Info("TorVM controller exiting")
	} else {
		// GUI mode: Fyne event loop on main thread, lifecycle in goroutine.
		ring := logging.NewRingWriter(1000)
		logger.AddWriter(ring)

		engine := lifecycle.NewEngine(cfg, logger)
		engine.Metrics = recorder
		engineRef = engine

		// Start config file watcher for hot reload in GUI mode.
		if *configFile != "" {
			watcher, wErr := config.NewConfigWatcher(*configFile, func(newCfg *config.Config) {
				diff := config.Diff(engine.Config, newCfg)
				if !diff.HasChanges() {
					return
				}
				for _, field := range diff.RestartRequired {
					logger.Info("config watcher: %s changed, restart required", field)
				}
				if len(diff.HotReloadable) > 0 {
					if rErr := engine.ReloadConfig(newCfg); rErr != nil {
						logger.Error("config watcher: reload failed: %v", rErr)
					}
				}
			})
			if wErr != nil {
				logger.Error("config watcher: %v", wErr)
			} else {
				defer watcher.Close()
			}
		}

		app := gui.New(cfg, engine, logger, ring, *configFile)

		// Signal handler for graceful shutdown in GUI mode.
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			sig := <-sigCh
			logger.Info("received signal %v, shutting down GUI", sig)
			app.RequestShutdown()
		}()

		app.Run()
	}
}

// queryStatus connects to a running TorVM instance and prints its status.
// Returns 0 if running, 1 if not running or error.
func queryStatus(cfg *config.Config) int {
	vmAddr := fmt.Sprintf("%s:%d", cfg.VMIP, cfg.ControlPort)

	// Check if VM control port is reachable.
	conn, err := net.DialTimeout("tcp", vmAddr, 3*time.Second)
	if err != nil {
		fmt.Println("TorVM Status: Stopped")
		fmt.Printf("  (could not reach %s)\n", vmAddr)
		return 1
	}
	if tc, ok := conn.(*net.TCPConn); ok {
		tc.SetLinger(0)
	}
	conn.Close()

	fmt.Println("TorVM Status: Running")
	fmt.Printf("  SOCKS Port: %d\n", cfg.SOCKSPort)

	// Try to get bootstrap status via Tor Control.
	ctrlAddr := fmt.Sprintf("%s:%d", cfg.VMIP, cfg.ControlPort)
	client, err := tor.NewControlClient(ctrlAddr, 5*time.Second)
	if err == nil {
		defer client.Close()
		if err := client.Authenticate(""); err == nil {
			status, err := client.GetBootstrapStatus()
			if err == nil {
				fmt.Printf("  Bootstrap: %d%% - %s\n", status.Progress, status.Summary)
			}
		}
	}

	return 0
}
