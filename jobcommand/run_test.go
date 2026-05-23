package jobcommand

import "testing"

func TestCommandAllowed(t *testing.T) {
	allowed := []string{"command_a", "command_b"}

	if !commandAllowed(allowed, "command_a") {
		t.Fatal("expected command_a to be allowed")
	}
	if commandAllowed(allowed, "command_c") {
		t.Fatal("expected command_c to be rejected")
	}
	if commandAllowed(nil, "command_a") {
		t.Fatal("expected nil allow-list to reject")
	}
}
