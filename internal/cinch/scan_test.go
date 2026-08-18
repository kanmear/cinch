package cinch

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fencedLines(t *testing.T, data string) []string {
	t.Helper()
	var got []string
	_ = forEachFencedLine([]byte(data), func(lineNo int, line string) {
		got = append(got, line)
	})
	return got
}

func TestForEachFencedLineSkipsCodeBlocks(t *testing.T) {
	cases := []struct {
		name string
		data string
		want []string
	}{
		{"no fence", "one\ntwo\n", []string{"one", "two"}},
		{"language tag", "one\n```go\ncode\n```\ntwo\n", []string{"one", "two"}},
		{"indented fence", "a\n   ```\nx\n   ```\nb\n", []string{"a", "b"}},
		{"multiple fences", "a\n```\n1\n```\nb\n```\n2\n```\nc\n", []string{"a", "b", "c"}},
		{"unterminated fence", "a\n```\nb\n", []string{"a"}},
		{"crlf", "one\r\ntwo\r\n", []string{"one", "two"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := fencedLines(t, tc.data)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("forEachFencedLine = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestForEachFencedLineKeepsLineNumbers(t *testing.T) {
	var lines []int
	_ = forEachFencedLine([]byte("1\n2\n```\n3\n4\n```\n7\n"), func(lineNo int, line string) {
		lines = append(lines, lineNo)
	})
	want := []int{1, 2, 7}
	if !reflect.DeepEqual(lines, want) {
		t.Fatalf("line numbers = %v, want %v", lines, want)
	}
}

// unreadableTree returns a temp dir whose sub/ subtree is unreadable,
// skipping the test when running as root (permission checks are void).
func unreadableTree(t *testing.T) string {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("running as root; permission checks are void")
	}
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "a.md"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sub, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(sub, 0o755) })
	return dir
}

func TestCollectMarkdownPropagatesWalkError(t *testing.T) {
	root := unreadableTree(t)
	var got []string
	_, err := collectMarkdown(root, func(path string) ([]string, error) {
		got = append(got, path)
		return nil, nil
	})
	if err == nil {
		t.Fatal("collectMarkdown = nil error, want error for unreadable subdirectory")
	}
	if !strings.Contains(err.Error(), "sub") {
		t.Fatalf("error %q does not mention the unreadable subdirectory", err)
	}
	for _, p := range got {
		if strings.Contains(p, "sub") {
			t.Fatalf("walk visited %q under the failed subdirectory", p)
		}
	}
}

func TestCollectMarkdownReadError(t *testing.T) {
	root := t.TempDir()
	broken := filepath.Join(root, "broken.md")
	if err := os.Symlink(filepath.Join(root, "missing.md"), broken); err != nil {
		t.Fatal(err)
	}
	_, err := collectMarkdown(root, func(path string) ([]string, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return []string{string(data)}, nil
	})
	if err == nil {
		t.Fatal("collectMarkdown = nil error, want error for unreadable file")
	}
}
