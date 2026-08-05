package check

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"cinch/internal/manifest"
)

func TestOwnsList(t *testing.T) {
	cases := []struct {
		name, text string
		want       []string
	}{
		{"present", "---\nrule_prefix: TAB\nowns:\n  - backend/tab.go\n  - templates/\n---\n\n# D\n", []string{"backend/tab.go", "templates/"}},
		{"owns before rule_prefix", "---\nowns:\n  - a.go\nrule_prefix: TAB\n---\n\n# D\n", []string{"a.go"}},
		{"no owns key", "---\nrule_prefix: TAB\n---\n\n# D\n", nil},
		{"no front matter", "# D\n", nil},
		{"body list is not owns", "---\nowns:\n  - a.go\n---\n\n# D\n\n- body item\n", []string{"a.go"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ownsList(c.text)
			if len(got) != len(c.want) {
				t.Fatalf("ownsList() = %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("ownsList() = %v, want %v", got, c.want)
				}
			}
		})
	}
}

// gitInit makes a hermetic temp git repo with the given files written but
// nothing committed (GIT_CONFIG_GLOBAL=/dev/null so the user's hooks, signing,
// and identity settings cannot interfere; identity comes from env).
func gitInit(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitEnv := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null",
			"GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=cinch test",
			"GIT_AUTHOR_EMAIL=test@cinch",
			"GIT_COMMITTER_NAME=cinch test",
			"GIT_COMMITTER_EMAIL=test@cinch")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	gitEnv("init", "-q", "-b", "main")
	return root
}

func gitCommitAll(t *testing.T, root string) {
	t.Helper()
	cmd := exec.Command("git", "-C", root, "add", "-A")
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=cinch test", "GIT_AUTHOR_EMAIL=test@cinch",
		"GIT_COMMITTER_NAME=cinch test", "GIT_COMMITTER_EMAIL=test@cinch")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add -A: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "-C", root, "commit", "-q", "-m", "initial")
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=cinch test", "GIT_AUTHOR_EMAIL=test@cinch",
		"GIT_COMMITTER_NAME=cinch test", "GIT_COMMITTER_EMAIL=test@cinch")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
}

func gitWrite(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const tabsDoc = "---\nrule_prefix: TAB\nowns:\n  - backend/tab.go\n---\n\n# Tabs Business Rules\n"

func runDiffCoupling(t *testing.T, root string) []finding {
	t.Helper()
	m := &manifest.Manifest{Vars: map[string]string{"paths.domain": ".agent/domain"}}
	r := &report{}
	checkDiffCoupling(root, m, r)
	return r.findings
}

func TestCheckDiffCoupling(t *testing.T) {

	files := map[string]string{
		".agent/domain/tabs.md": tabsDoc,
		"backend/tab.go":        "v1\n",
	}

	t.Run("warns when owns path changes without the doc", func(t *testing.T) {
		root := gitInit(t, files)
		gitCommitAll(t, root)
		gitWrite(t, root, "backend/tab.go", "v2\n")
		fs := runDiffCoupling(t, root)
		hasFinding(t, fs, "C10", "warn", "backend/tab.go")
		hasFinding(t, fs, "C10", "warn", "tabs.md")
	})

	t.Run("staged change counts too", func(t *testing.T) {
		root := gitInit(t, files)
		gitCommitAll(t, root)
		gitWrite(t, root, "backend/tab.go", "v2\n")
		cmd := exec.Command("git", "-C", root, "add", "backend/tab.go")
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git add: %v\n%s", err, out)
		}
		fs := runDiffCoupling(t, root)
		hasFinding(t, fs, "C10", "warn", "backend/tab.go")
	})

	t.Run("doc in the same change is silent", func(t *testing.T) {
		root := gitInit(t, files)
		gitCommitAll(t, root)
		gitWrite(t, root, "backend/tab.go", "v2\n")
		gitWrite(t, root, ".agent/domain/tabs.md", tabsDoc+"\n## Tabs History\n")
		if fs := runDiffCoupling(t, root); len(fs) != 0 {
			t.Fatalf("expected green, got %v", fs)
		}
	})

	t.Run("unrelated change is silent", func(t *testing.T) {
		root := gitInit(t, files)
		gitCommitAll(t, root)
		gitWrite(t, root, "README.md", "readme\n")
		if fs := runDiffCoupling(t, root); len(fs) != 0 {
			t.Fatalf("expected green, got %v", fs)
		}
	})

	t.Run("directory owns entry matches nested paths", func(t *testing.T) {
		dirFiles := map[string]string{
			".agent/domain/harness.md": "---\nrule_prefix: HARNESS\nowns:\n  - templates/\n---\n\n# Harness Domain Rules\n",
		}
		root := gitInit(t, dirFiles)
		gitCommitAll(t, root)
		gitWrite(t, root, "templates/plan-feature.md", "changed\n")
		cmd := exec.Command("git", "-C", root, "add", "-A")
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git add: %v\n%s", err, out)
		}
		fs := runDiffCoupling(t, root)
		hasFinding(t, fs, "C10", "warn", "templates/plan-feature.md")
	})

	t.Run("domain doc only change is silent", func(t *testing.T) {
		root := gitInit(t, files)
		gitCommitAll(t, root)
		gitWrite(t, root, ".agent/domain/tabs.md", tabsDoc+"\n## Tabs History\n")
		if fs := runDiffCoupling(t, root); len(fs) != 0 {
			t.Fatalf("expected green, got %v", fs)
		}
	})

	t.Run("not a git repo skips", func(t *testing.T) {
		root := t.TempDir()
		p := filepath.Join(root, ".agent/domain")
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "tabs.md"), []byte(tabsDoc), 0o644); err != nil {
			t.Fatal(err)
		}
		if fs := runDiffCoupling(t, root); len(fs) != 0 {
			t.Fatalf("expected skip, got %v", fs)
		}
	})

	t.Run("no commits yet skips", func(t *testing.T) {
		root := gitInit(t, files)
		if fs := runDiffCoupling(t, root); len(fs) != 0 {
			t.Fatalf("expected skip (unborn HEAD), got %v", fs)
		}
	})

	t.Run("clean tree is silent", func(t *testing.T) {
		root := gitInit(t, files)
		gitCommitAll(t, root)
		if fs := runDiffCoupling(t, root); len(fs) != 0 {
			t.Fatalf("expected green, got %v", fs)
		}
	})

	t.Run("no domain layer skips", func(t *testing.T) {
		root := gitInit(t, map[string]string{"backend/tab.go": "v1\n"})
		gitCommitAll(t, root)
		r := &report{}
		checkDiffCoupling(root, &manifest.Manifest{Vars: map[string]string{}}, r)
		if len(r.findings) != 0 {
			t.Fatalf("expected skip, got %v", r.findings)
		}
	})

	t.Run("domain doc with no owns skips", func(t *testing.T) {
		noOwns := map[string]string{
			".agent/domain/tabs.md": "---\nrule_prefix: TAB\n---\n\n# Tabs Business Rules\n",
			"backend/tab.go":        "v1\n",
		}
		root := gitInit(t, noOwns)
		gitCommitAll(t, root)
		gitWrite(t, root, "backend/tab.go", "v2\n")
		if fs := runDiffCoupling(t, root); len(fs) != 0 {
			t.Fatalf("expected skip (no owns list), got %v", fs)
		}
	})

	t.Run("one warn per domain, not per path", func(t *testing.T) {
		multi := map[string]string{
			".agent/domain/tabs.md": "---\nrule_prefix: TAB\nowns:\n  - backend/tab.go\n  - backend/models/tab.go\n---\n\n# Tabs Business Rules\n",
			"backend/tab.go":        "v1\n",
			"backend/models/tab.go": "v1\n",
		}
		root := gitInit(t, multi)
		gitCommitAll(t, root)
		gitWrite(t, root, "backend/tab.go", "v2\n")
		gitWrite(t, root, "backend/models/tab.go", "v2\n")
		fs := runDiffCoupling(t, root)
		warns := 0
		for _, f := range fs {
			if f.code == "C10" {
				warns++
			}
		}
		if warns != 1 {
			t.Fatalf("want exactly one C10 warn, got %v", fs)
		}
	})
}
