package cinch

import "testing"

func TestTitleAndTrigger(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantTitle   string
		wantTrigger string
	}{
		{
			name:        "plain sentence trigger",
			body:        "# Domain Rule Maintenance\n\nAdd, edit, or reorganize domain rules in `.docs/domain/`.\n\n## Usage\n",
			wantTitle:   "Domain Rule Maintenance",
			wantTrigger: "Add, edit, or reorganize domain rules in `.docs/domain/`.",
		},
		{
			name:        "bold-prefixed trigger, no blank line before it",
			body:        "# Philosophy: Lean, Scalable Documentation\n**The Core Insight:** With clean architecture, the code IS the documentation.\n",
			wantTitle:   "Philosophy: Lean, Scalable Documentation",
			wantTrigger: "**The Core Insight:** With clean architecture, the code IS the documentation.",
		},
		{
			name:        "no H1 at all",
			body:        "no title here\njust text\n",
			wantTitle:   "",
			wantTrigger: "",
		},
		{
			name:        "sentence hand-wrapped across physical lines joins",
			body:        "# Plan Execution Workflow\n\nExecute the tasks in an already-decomposed plan file — one atomic task per session, git\nconventions enforced, and a Session Handoff trail so work resumes cold.\n\n---\n",
			wantTitle:   "Plan Execution Workflow",
			wantTrigger: "Execute the tasks in an already-decomposed plan file — one atomic task per session, git conventions enforced, and a Session Handoff trail so work resumes cold.",
		},
		{
			name:        "stops before a bullet immediately following, no blank line between",
			body:        "# Documentation Sync\n\nUpdate `.docs/` documentation to reflect recent code changes.\n- **Note:** more detail here.\n",
			wantTitle:   "Documentation Sync",
			wantTrigger: "Update `.docs/` documentation to reflect recent code changes.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, trigger := titleAndTrigger(tt.body)
			if title != tt.wantTitle {
				t.Fatalf("titleAndTrigger: title want %q, got %q", tt.wantTitle, title)
			}
			if trigger != tt.wantTrigger {
				t.Fatalf("titleAndTrigger: trigger want %q, got %q", tt.wantTrigger, trigger)
			}
		})
	}
}
