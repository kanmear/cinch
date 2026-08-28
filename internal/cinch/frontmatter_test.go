package cinch

import (
	"reflect"
	"testing"
)

func TestParseFrontmatterAbsent(t *testing.T) {
	got, err := parseFrontmatter([]byte("# Title\n\nno frontmatter here\n"))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !reflect.DeepEqual(got, docFrontmatter{}) {
		t.Fatalf("got = %+v, want zero value", got)
	}
}

func TestParseFrontmatterOwnsAndPrefix(t *testing.T) {
	body := "---\nowns:\n  - internal/cinch\n  - main.go\nrule_prefix: SIG-\n---\n# Title\n\nbody\n"
	got, err := parseFrontmatter([]byte(body))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	want := docFrontmatter{Owns: []string{"internal/cinch", "main.go"}, RulePrefix: "SIG-"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got = %+v, want %+v", got, want)
	}
}

func TestParseFrontmatterUnterminated(t *testing.T) {
	body := "---\nowns:\n  - internal/cinch\n\n# Title, no closing fence\n"
	got, err := parseFrontmatter([]byte(body))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !reflect.DeepEqual(got, docFrontmatter{}) {
		t.Fatalf("got = %+v, want zero value for an unterminated block", got)
	}
}

func TestParseFrontmatterMalformedYAML(t *testing.T) {
	body := "---\nowns: \"not-a-list\"\n---\n# Title\n"
	_, err := parseFrontmatter([]byte(body))
	if err == nil {
		t.Fatal("err = nil, want an error for owns: given as a scalar instead of a list")
	}
}

func TestParseFrontmatterFileMissing(t *testing.T) {
	_, err := parseFrontmatterFile("/nonexistent/path/does-not-exist.md")
	if err == nil {
		t.Fatal("err = nil, want an error for a missing file")
	}
}

func TestParseFrontmatterFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "domain.md", "---\nrule_prefix: SIG-\n---\n# Signatures\n")
	got, err := parseFrontmatterFile(path)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if got.RulePrefix != "SIG-" {
		t.Fatalf("RulePrefix = %q, want %q", got.RulePrefix, "SIG-")
	}
}
