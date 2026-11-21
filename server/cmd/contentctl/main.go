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
	"syscall"

	"github.com/maillunex-lang/NewMMO/server/internal/content"
)

const usage = `usage: contentctl verify <dir>
       contentctl stage <dir>
       contentctl publish <build_id> [--expected-current <id>]
       contentctl rollback <build_id> --expected-current <id>
       contentctl status`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:], os.Getenv, os.Stdout)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, getenv func(string) string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New(usage)
	}
	command := args[0]
	var expectedCurrent string
	switch command {
	case "verify", "stage":
		if len(args) != 2 {
			return errors.New(usage)
		}
	case "publish", "rollback":
		if len(args) < 2 {
			return errors.New(usage)
		}
		flags := flag.NewFlagSet(command, flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		flags.StringVar(&expectedCurrent, "expected-current", "", "expected active build ID")
		if command == "publish" {
			flags.StringVar(&expectedCurrent, "expected-previous", "", "alias for --expected-current")
		}
		if err := flags.Parse(args[2:]); err != nil {
			return err
		}
		if flags.NArg() != 0 || flags.NFlag() > 1 || (command == "rollback" && flags.NFlag() != 1) {
			return errors.New(usage)
		}
	case "status":
		if len(args) != 1 {
			return errors.New(usage)
		}
	case "help", "-h", "--help":
		_, err := fmt.Fprintln(out, usage)
		return err
	default:
		return errors.New(usage)
	}

	encoder := json.NewEncoder(out)
	if command == "verify" {
		summary, err := content.Verify(args[1])
		if err != nil {
			return err
		}
		return encoder.Encode(summary)
	}

	url := getenv("NEWMMO_DATABASE_URL")
	if url == "" {
		return errors.New("NEWMMO_DATABASE_URL is required")
	}
	store, err := content.Open(ctx, url, "contentctl")
	if err != nil {
		return err
	}
	defer store.Close()

	switch command {
	case "status":
		status, err := store.Status(ctx)
		if err != nil {
			return err
		}
		return encoder.Encode(status)
	case "stage":
		summary, err := store.Stage(ctx, args[1])
		if err != nil {
			return err
		}
		return encoder.Encode(summary)
	case "publish":
		err = store.Publish(ctx, args[1], expectedCurrent)
	case "rollback":
		err = store.Rollback(ctx, args[1], expectedCurrent)
	}
	if err != nil {
		return err
	}
	return encoder.Encode(struct {
		BuildID string `json:"build_id"`
	}{BuildID: args[1]})
}
