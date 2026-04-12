package tests

import (
	"strings"
	"testing"

	"japan_data_project/internal/app/runner"
)

func TestRunnerRun_NoArgsReturnsUsage(t *testing.T) {
	err := runner.Run(nil)
	if err == nil {
		t.Fatal("expected usage error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "usage: app <api|download|ingestor>") {
		t.Fatalf("usage message mismatch: %q", msg)
	}
}

func TestRunnerRun_HelpReturnsUsage(t *testing.T) {
	err := runner.Run([]string{"help"})
	if err == nil {
		t.Fatal("expected usage error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "usage: app <api|download|ingestor>") {
		t.Fatalf("usage message mismatch: %q", msg)
	}
}

func TestRunnerRun_UnknownSubcommand(t *testing.T) {
	err := runner.Run([]string{"unknown"})
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
	msg := err.Error()
	if !strings.Contains(msg, "unknown subcommand: unknown") {
		t.Fatalf("missing unknown-subcommand message: %q", msg)
	}
	if !strings.Contains(msg, "usage: app <api|download|ingestor>") {
		t.Fatalf("missing usage message for unknown subcommand: %q", msg)
	}
}
