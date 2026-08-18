package cinch

import (
	"reflect"
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
