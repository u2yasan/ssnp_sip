package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/u2yasan/ssnp_sip/portal/internal/server"
)

func main() {
	if err := runMain(); err != nil {
		fmt.Fprintf(os.Stderr, "portal-server: %v\n", err)
		os.Exit(1)
	}
}

func runMain() error {
	listenAddr := flag.String("listen", "127.0.0.1:8080", "listen address")
	policyPath := flag.String("policy", "", "path to policy yaml")
	clockSkew := flag.Int("allowed-clock-skew-seconds", 300, "allowed timestamp clock skew in seconds")
	flag.Parse()

	if *policyPath == "" {
		return fmt.Errorf("missing --policy")
	}

	srv, err := server.New(server.Config{
		ListenAddr:              *listenAddr,
		PolicyPath:              *policyPath,
		AllowedClockSkewSeconds: *clockSkew,
	})
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return srv.ListenAndServe(ctx)
}
