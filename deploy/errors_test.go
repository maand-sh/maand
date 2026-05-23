package deploy

import (
	"errors"
	"testing"
)

func TestJobError_ErrorAndUnwrap(t *testing.T) {
	inner := errors.New("inner")
	err := &JobError{Job: "app", Err: inner}
	if err.Error() != "job app: inner" {
		t.Fatalf("got %q", err.Error())
	}
	if !errors.Is(err, inner) {
		t.Fatal("expected unwrap")
	}
}

func TestJoinErrors_empty(t *testing.T) {
	if joinErrors("prefix", nil) != nil {
		t.Fatal("expected nil")
	}
}

func TestJoinErrors_prefix(t *testing.T) {
	err := joinErrors("deploy failed", []error{errors.New("a"), errors.New("b")})
	if err == nil || err.Error() != "deploy failed:\na\nb" {
		t.Fatalf("got %v", err)
	}
}
