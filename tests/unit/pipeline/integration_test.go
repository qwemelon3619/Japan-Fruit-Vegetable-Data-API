package tests

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func requirePipelineIntegration(t *testing.T) {
	t.Helper()
	if strings.TrimSpace(strings.ToLower(os.Getenv("RUN_PIPELINE_INTEGRATION_TESTS"))) != "1" {
		t.Skip("set RUN_PIPELINE_INTEGRATION_TESTS=1 to run pipeline integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "compose", "ps", "--status", "running")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Skipf("docker compose not available/running: %v (%s)", err, strings.TrimSpace(stderr.String()))
	}
}

func runCmd(t *testing.T, timeout time.Duration, name string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("command failed: %s %s: %v\nstderr: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String()
}

func TestPipeline_CrontabIncludesExpectedJobs(t *testing.T) {
	requirePipelineIntegration(t)
	out := runCmd(t, 15*time.Second, "docker", "compose", "exec", "-T", "pipeline-cron", "sh", "-lc", "cat /etc/crontabs/root")
	if !strings.Contains(out, "daily_download_ingest.sh") {
		t.Fatalf("daily ingest cron job missing in crontab:\n%s", out)
	}
	if !strings.Contains(out, "monitor_snapshot.sh") {
		t.Fatalf("monitor snapshot cron job missing in crontab:\n%s", out)
	}
}

func TestPipeline_MonitorSnapshotWritesLatestRow(t *testing.T) {
	requirePipelineIntegration(t)
	_ = runCmd(t, 20*time.Second, "docker", "compose", "exec", "-T", "pipeline-cron", "sh", "-lc", "/usr/local/bin/monitor_snapshot.sh")
	row := strings.TrimSpace(runCmd(t, 15*time.Second, "docker", "compose", "exec", "-T", "pipeline-cron", "sh", "-lc", "tail -n 1 /data/monitoring/csv/snapshots.csv"))
	if row == "" {
		t.Fatal("snapshots.csv last row is empty")
	}
	cols := strings.Split(row, ",")
	if len(cols) != 7 {
		t.Fatalf("snapshots row column count mismatch: got=%d row=%q", len(cols), row)
	}
	if cols[0] == "" {
		t.Fatalf("timestamp is empty: row=%q", row)
	}
	if cols[1] != "0" && cols[1] != "1" {
		t.Fatalf("ready_ok must be 0 or 1: row=%q", row)
	}
}
