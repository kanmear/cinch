package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCatchMatrixIsCurrent(t *testing.T) {
	root := filepath.Join("..", "..")
	want, err := generate(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, "bench", "catch-matrix.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatal("bench/catch-matrix.md is stale; run `make catch-matrix` and commit the result")
	}
}

func TestCollectRequiresSeededAnnotations(t *testing.T) {
	dir := t.TempDir()
	body := "package x\n\nimport \"testing\"\n\nfunc TestSeededDefectX(t *testing.T) {}\n"
	if err := os.WriteFile(filepath.Join(dir, "x_test.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := collect(dir); err == nil || !strings.Contains(err.Error(), "TestSeededDefectX") {
		t.Fatalf("collect = %v, want an error naming the unannotated seeded test", err)
	}
}

func TestParseAnnotationRejectsUnknownValues(t *testing.T) {
	for _, line := range []string{
		"rules | caught",
		"lint | caught | x",
		"rules | missed | x",
		"rules | caught | ",
	} {
		if _, err := parseAnnotation(line); err == nil {
			t.Errorf("parseAnnotation(%q) = nil error, want one", line)
		}
	}
	e, err := parseAnnotation(" rules | gap | hollowed test ")
	if err != nil || e.check != "rules" || e.verdict != "gap" || e.defect != "hollowed test" {
		t.Fatalf("parseAnnotation = %+v, %v", e, err)
	}
}
