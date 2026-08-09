package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunVersion(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := run([]string{"version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run() code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "spec 0.2.0") {
		t.Fatalf("version output = %q", stdout.String())
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := run([]string{"pretend-success"}, &stdout, &stderr); code != exUsage {
		t.Fatalf("run() code = %d, want %d", code, exUsage)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunEngineExplainRejectsUnknownCandidate(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := run([]string{"engine", "explain", "not-real"}, &stdout, &stderr); code != exUsage {
		t.Fatalf("run() code = %d, want %d", code, exUsage)
	}
	if !strings.Contains(stderr.String(), "unknown backend candidate") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
