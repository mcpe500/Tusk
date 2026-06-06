package main

import "testing"

func TestAsStringSlice(t *testing.T) {
	got, err := asStringSlice([]interface{}{"a", "b"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("unexpected result: %#v", got)
	}

	if _, err := asStringSlice(123); err == nil {
		t.Fatal("expected error for non-array input")
	}

	if got, err := asStringSlice([]string{"x", "y"}); err != nil || len(got) != 2 {
		t.Fatalf("expected parsed []string, got %#v err=%v", got, err)
	}
}

func TestAsStringSliceInvalidValues(t *testing.T) {
	if _, err := asStringSlice([]interface{}{"ok", 2}); err == nil {
		t.Fatal("expected error for mixed types")
	}
}

func TestReqParams(t *testing.T) {
	if p, ok := reqParams(map[string]interface{}{}); !ok || p == nil {
		t.Fatalf("expected empty params for missing field, got ok=%v", ok)
	}

	if _, ok := reqParams(map[string]interface{}{"params": 1}); ok {
		t.Fatalf("expected invalid params for non-map")
	}

	p, ok := reqParams(map[string]interface{}{"params": map[string]interface{}{"x": 1}})
	if !ok || p["x"] != 1 {
		t.Fatalf("expected parsed map params, got ok=%v p=%#v", ok, p)
	}
}

func TestGetStringOrDefault(t *testing.T) {
	if got := getStringOrDefault("ok"); got != "ok" {
		t.Fatalf("unexpected: %q", got)
	}
	if got := getStringOrDefault(123); got != "" {
		t.Fatalf("expected empty for non-string")
	}
	if got := getStringOrDefault(nil); got != "" {
		t.Fatalf("expected empty for nil")
	}
}
