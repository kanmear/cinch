package cinch

import "testing"

func TestTitleAndTrigger(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		wantTitle   string
		wantTrigger string
	}{
		{"no h1", "plain text\n", "", ""},
		{"title only", "# T", "T", ""},
		{"basic", "# Title\nRun the thing.", "Title", "Run the thing."},
		{"no punctuation", "# Title\na\nb", "Title", "a b"},
		{"blank lines after title", "# Title\n\n\nRun it.\n", "Title", "Run it."},
		{"paragraph break ends trigger", "# Title\na\n\nb", "Title", "a"},
		{"sentence end", "# Title\nFirst sentence.\nSecond.", "Title", "First sentence."},
		{"heading breaks trigger", "# Title\nintro\n## Sub\nmore", "Title", "intro"},
		{"list breaks trigger", "# Title\nintro\n- item", "Title", "intro"},
		{"code fence breaks trigger", "# Title\nintro\n```\ncode\n```", "Title", "intro"},
		{"numbered list breaks trigger", "# Title\nintro\n1. item", "Title", "intro"},
		{"crlf", "# Title\r\nRun it.\r\n", "Title", "Run it."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			title, trigger := titleAndTrigger(tc.body)
			if title != tc.wantTitle || trigger != tc.wantTrigger {
				t.Fatalf("titleAndTrigger = (%q, %q), want (%q, %q)", title, trigger, tc.wantTitle, tc.wantTrigger)
			}
		})
	}
}
