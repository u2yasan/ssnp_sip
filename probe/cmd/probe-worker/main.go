package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/u2yasan/ssnp_sip/probe/internal/config"
	"github.com/u2yasan/ssnp_sip/probe/internal/worker"
)

func main() {
	if err := runMain(); err != nil {
		fmt.Fprintf(os.Stderr, "probe-worker: %v\n", err)
		os.Exit(1)
	}
}

func runMain() error {
	var configPath string
	flag.StringVar(&configPath, "config", "", "path to config yaml")
	flag.Parse()

	if configPath == "" {
		return errors.New("missing --config")
	}

	args := flag.Args()
	if len(args) == 0 {
		return errors.New("missing command: run | run-once")
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	w := worker.New(cfg, log.New(os.Stderr, "", log.LstdFlags))
	switch args[0] {
	case "run":
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		err := w.Run(ctx)
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	case "run-once":
		return w.RunOnce(context.Background())
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}
