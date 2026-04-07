package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	nodesConfigPath := flag.String("nodes-config", "", "path to known node seed config")
	statePath := flag.String("state-path", "", "path to runtime state snapshot json")
	clockSkew := flag.Int("allowed-clock-skew-seconds", 300, "allowed timestamp clock skew in seconds")
	emailTo := flag.String("email-to", "", "fallback notification email recipient")
	smtpHost := flag.String("smtp-host", "", "smtp host")
	smtpPort := flag.Int("smtp-port", 587, "smtp port")
	smtpUsername := flag.String("smtp-username", "", "smtp username")
	smtpFrom := flag.String("smtp-from", "", "smtp from address")
	staleAfter := flag.Int("heartbeat-stale-after-seconds", 900, "seconds after last heartbeat before stale alert")
	failedAfter := flag.Int("heartbeat-failed-after-seconds", 1800, "seconds after last heartbeat before failed alert")
	alertScan := flag.Int("alert-scan-interval-seconds", 60, "seconds between heartbeat alert scans")
	flag.Parse()

	if *policyPath == "" {
		return fmt.Errorf("missing --policy")
	}
	if *nodesConfigPath == "" {
		return fmt.Errorf("missing --nodes-config")
	}
	if *statePath == "" {
		return fmt.Errorf("missing --state-path")
	}

	srv, err := server.New(server.Config{
		ListenAddr:              *listenAddr,
		PolicyPath:              *policyPath,
		NodesConfigPath:         *nodesConfigPath,
		StatePath:               *statePath,
		AllowedClockSkewSeconds: *clockSkew,
		NotificationEmailTo:     *emailTo,
		SMTPHost:                *smtpHost,
		SMTPPort:                *smtpPort,
		SMTPUsername:            *smtpUsername,
		SMTPPassword:            os.Getenv("SSNP_SMTP_PASSWORD"),
		SMTPFrom:                *smtpFrom,
		HeartbeatStaleAfter:     time.Duration(*staleAfter) * time.Second,
		HeartbeatFailedAfter:    time.Duration(*failedAfter) * time.Second,
		AlertScanInterval:       time.Duration(*alertScan) * time.Second,
	})
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return srv.ListenAndServe(ctx)
}
