package types

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestBuildSpecStringShorthand(t *testing.T) {
	input := `
services:
  app:
    build: .
`
	var spec ComposeSpec
	if err := yaml.Unmarshal([]byte(input), &spec); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	svc, ok := spec.Services["app"]
	if !ok {
		t.Fatal("missing service 'app'")
	}
	if svc.Build == nil {
		t.Fatal("Build is nil")
	}
	if svc.Build.Context != "." {
		t.Errorf("Context = %q, want %q", svc.Build.Context, ".")
	}
}

func TestBuildSpecObjectForm(t *testing.T) {
	input := `
services:
  app:
    build:
      context: ./api
      dockerfile: Dockerfile.dev
`
	var spec ComposeSpec
	if err := yaml.Unmarshal([]byte(input), &spec); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	svc := spec.Services["app"]
	if svc.Build == nil {
		t.Fatal("Build is nil")
	}
	if svc.Build.Context != "./api" {
		t.Errorf("Context = %q, want %q", svc.Build.Context, "./api")
	}
	if svc.Build.Dockerfile != "Dockerfile.dev" {
		t.Errorf("Dockerfile = %q, want %q", svc.Build.Dockerfile, "Dockerfile.dev")
	}
}

func TestBuildSpecNilWhenOmitted(t *testing.T) {
	input := `
services:
  app:
    image: nginx
`
	var spec ComposeSpec
	if err := yaml.Unmarshal([]byte(input), &spec); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if spec.Services["app"].Build != nil {
		t.Error("Build should be nil when omitted")
	}
}
