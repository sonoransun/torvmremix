package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/user/extorvm/controller/gui"
	"github.com/user/extorvm/controller/internal/config"
	"github.com/user/extorvm/controller/internal/lifecycle"
	"github.com/user/extorvm/controller/internal/logging"
	"github.com/user/extorvm/controller/internal/platform"
)

func main() {
	var (
		accelFlag   = flag.String("accel", "", "acceleration backend: kvm, hvf, whpx, tcg")
		verboseFlag = flag.Bool("verbose", false, "enable debug logging")
		headless    = flag.Bool("headless", false, "run without GUI")
		configFile  = flag.String("config", "", "path to JSON config file")
		clean       = flag.Bool("clean", false, "remove state disk before starting")
		replace     = flag.Bool("replace", false, "replace existing state disk with fresh copy")
	)
	flag.Parse()

	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: load config: %v\n", err)
		os.Exit(1)
	}

	cfg.Verbose = *verboseFlag

	// Detect or set acceleration.
	if *accelFlag != "" {
		accel, err := platform.ParseAccel(*accelFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		cfg.Accel = string(accel)
	} else {
		info, _ := platform.DetectAccel()
		cfg.Accel = string(info.Accel)
	}

	logger, err := logging.NewLogger(logging.Options{
		Verbose: cfg.Verbose,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: create logger: %v\n", err)
		os.Exit(1)
	}

	logger.Info("TorVM controller starting (accel=%s)", cfg.Accel)

	// Handle --clean: remove state disk.
	if *clean || *replace {
		logger.Info("removing state disk: %s", cfg.StateDiskPath)
		os.Remove(cfg.StateDiskPath)
	}

	if *headless {
		// CLI mode: existing blocking lifecycle.
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			sig := <-sigCh
			logger.Info("received signal %v, shutting down", sig)
			cancel()
		}()

		engine := lifecycle.NewEngine(cfg, logger)
		if err := engine.Run(ctx); err != nil {
			logger.Error("lifecycle error: %v", err)
			os.Exit(1)
		}

		logger.Info("TorVM controller exiting")
	} else {
		// GUI mode: Fyne event loop on main thread, lifecycle in goroutine.
		ring := logging.NewRingWriter(1000)
		logger.AddWriter(ring)

		engine := lifecycle.NewEngine(cfg, logger)
		app := gui.New(cfg, engine, logger, ring, *configFile)
		app.Run()
	}
}
