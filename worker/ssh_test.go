package worker

import (
	"strings"
	"testing"
)

func TestShellQuote(t *testing.T) {
	if shellQuote("abc") != "'abc'" {
		t.Fatalf("got %q", shellQuote("abc"))
	}
	quoted := shellQuote("it's fine")
	if !strings.Contains(quoted, `'`) {
		t.Fatalf("expected escaped quote: %q", quoted)
	}
}

func TestSSHTargetIPv6(t *testing.T) {
	got := sshTarget("agent", "2001:db8::1")
	if got != "agent@[2001:db8::1]" {
		t.Fatalf("got %q", got)
	}
}

func TestSSHTargetIPv4(t *testing.T) {
	got := sshTarget("agent", "10.0.0.1")
	if got != "agent@10.0.0.1" {
		t.Fatalf("got %q", got)
	}
}

func TestRemoteShellCommand(t *testing.T) {
	cmd := remoteShellCommand(true)
	for _, want := range []string{"timeout 300", "sudo", "bash", "-s"} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("command %q missing %q", cmd, want)
		}
	}
	if strings.Contains(cmd, "env") {
		t.Fatalf("remote shell command should not duplicate env: %q", cmd)
	}
}

func TestSSHClientArgsPerWorker(t *testing.T) {
	args := SSHClientArgs("/bucket/secrets/worker.key", "10.0.0.1")
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"StrictHostKeyChecking=no",
		"ControlMaster=auto",
		"ControlPath=",
		"ssh/10_0_0_1",
		"-i",
		"/bucket/secrets/worker.key",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args %q missing %q", joined, want)
		}
	}
	if strings.Contains(joined, "%r") {
		t.Fatalf("control path should not use rsync-ambiguous %% tokens: %q", joined)
	}
}

func TestRSHShell(t *testing.T) {
	got := RSHShell("/bucket/secrets/worker.key", "10.0.0.2")
	if !strings.HasPrefix(got, "ssh ") {
		t.Fatalf("expected ssh prefix: %q", got)
	}
	if !strings.Contains(got, "-i /bucket/secrets/worker.key") {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, "ssh/10_0_0_2") {
		t.Fatalf("expected per-worker control socket: %q", got)
	}
}

func TestControlSocketName(t *testing.T) {
	if controlSocketName("10.48.200.4") != "10_48_200_4" {
		t.Fatalf("got %q", controlSocketName("10.48.200.4"))
	}
}
