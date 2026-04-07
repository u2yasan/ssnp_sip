package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/u2yasan/ssnp_sip/agent/internal/config"
	agentcrypto "github.com/u2yasan/ssnp_sip/agent/internal/crypto"
	"github.com/u2yasan/ssnp_sip/agent/internal/runtime"
)

func main() {
	if err := runMain(); err != nil {
		fmt.Fprintf(os.Stderr, "program-agent: %v\n", err)
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
		return errors.New("missing command: run | enroll | check | telemetry | gen-key")
	}

	if args[0] == "gen-key" {
		return runGenKey(args[1:], os.Stdout)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	agent, err := runtime.NewAgent(cfg)
	if err != nil {
		return err
	}

	switch args[0] {
	case "run":
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return agent.Run(ctx)
	case "enroll":
		fs := flag.NewFlagSet("enroll", flag.ContinueOnError)
		challengeID := fs.String("challenge-id", "", "enrollment challenge id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *challengeID == "" {
			return errors.New("missing --challenge-id")
		}
		return agent.Enroll(context.Background(), *challengeID)
	case "check":
		fs := flag.NewFlagSet("check", flag.ContinueOnError)
		eventType := fs.String("event-type", "", "registration | voting_key_renewal | recheck")
		eventID := fs.String("event-id", "", "check event id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *eventType == "" || *eventID == "" {
			return errors.New("missing --event-type or --event-id")
		}
		return agent.RunChecks(context.Background(), *eventType, *eventID)
	case "telemetry":
		fs := flag.NewFlagSet("telemetry", flag.ContinueOnError)
		var warningFlags stringListFlag
		fs.Var(&warningFlags, "warning-flag", "warning flag to submit (repeatable)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if len(warningFlags) == 0 {
			return errors.New("missing --warning-flag")
		}
		return agent.SubmitTelemetry(context.Background(), []string(warningFlags))
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func runGenKey(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("gen-key", flag.ContinueOnError)
	outDir := fs.String("out-dir", "./keys", "directory to write agent key pair")
	if err := fs.Parse(args); err != nil {
		return err
	}
	privateKeyPath, publicKeyPath, err := agentcrypto.GenerateAndWriteKeyPair(*outDir)
	if err != nil {
		return err
	}
	return json.NewEncoder(stdout).Encode(map[string]string{
		"private_key_path": privateKeyPath,
		"public_key_path":  publicKeyPath,
	})
}

type stringListFlag []string

func (s *stringListFlag) String() string {
	return strings.Join(*s, ",")
}

func (s *stringListFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("warning flag must not be empty")
	}
	*s = append(*s, value)
	return nil
}
