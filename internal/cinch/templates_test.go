package cinch

import (
	"io/fs"
	"path"
	"regexp"
	"strings"
	"testing"
)

const workflowPackLineBudget = 650

var workflowReferenceRe = regexp.MustCompile(`workflows/([A-Za-z0-9_-]+\.md)`)

// renderedWorkflows renders the pack with a default cinch.yml and returns
// only the workflow files, leaving out the hook shims.
func renderedWorkflows(t *testing.T) []renderFile {
	t.Helper()
	m, err := parseManifestBytes([]byte(""), manifestPath)
	if err != nil {
		t.Fatalf("parseManifestBytes: %v", err)
	}
	files, err := renderAll(m)
	if err != nil {
		t.Fatalf("renderAll: %v", err)
	}
	var workflows []renderFile
	for _, f := range files {
		if path.Base(path.Dir(f.destination)) == workflowsSubdir {
			workflows = append(workflows, f)
		}
	}
	if len(workflows) == 0 {
		t.Fatal("renderAll produced no workflows")
	}
	return workflows
}

// The pack used to point agents at a manifest.yml cinch never creates.
func TestWorkflowsNameNoUncreatedConfig(t *testing.T) {
	forbidden := []string{"manifest.yml", "manifest.", "taxonomy", "development.commands", "dev-task-primitive"}
	for _, f := range renderedWorkflows(t) {
		t.Run(path.Base(f.destination), func(t *testing.T) {
			for _, s := range forbidden {
				if strings.Contains(f.body, s) {
					t.Errorf("contains %q", s)
				}
			}
		})
	}
}

func TestWorkflowReferencesResolve(t *testing.T) {
	workflows := renderedWorkflows(t)
	produced := map[string]bool{}
	for _, f := range workflows {
		produced[path.Base(f.destination)] = true
	}
	for _, f := range workflows {
		t.Run(path.Base(f.destination), func(t *testing.T) {
			for _, m := range workflowReferenceRe.FindAllStringSubmatch(f.body, -1) {
				if !produced[m[1]] {
					t.Errorf("references workflows/%s, which the render doesn't produce", m[1])
				}
			}
		})
	}
}

func TestWorkflowPackLineBudget(t *testing.T) {
	total := strings.Count(philosophySrc, "\n")
	err := fs.WalkDir(templatesFS, templatesDirectory, func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(templatesFS, name)
		total += strings.Count(string(data), "\n")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if total > workflowPackLineBudget {
		t.Fatalf("templates plus philosophy = %d lines, budget is %d", total, workflowPackLineBudget)
	}
}

func TestWorkflowsHaveTitleAndTrigger(t *testing.T) {
	for _, f := range renderedWorkflows(t) {
		t.Run(path.Base(f.destination), func(t *testing.T) {
			title, trigger := titleAndTrigger(f.body)
			if title == "" || trigger == "" {
				t.Fatalf("title = %q, trigger = %q, want both non-empty", title, trigger)
			}
		})
	}
}

// Rendered workflows live under the docs root, where every numbered rule item
// is a real rule, and a sibling repo's workflows are scanned for markers when
// it is listed in rules.roots. An example outside a fence would plant one in
// every consumer.
func TestWorkflowsPlantNoRulesOrMarkers(t *testing.T) {
	for _, f := range renderedWorkflows(t) {
		t.Run(path.Base(f.destination), func(t *testing.T) {
			if items := parseRuleItems(f.destination, []byte(f.body)); len(items) != 0 {
				t.Errorf("parses as rules: %+v", items)
			}
			markers, err := scanMarkers(strings.NewReader(f.body), f.destination, true)
			if err != nil {
				t.Fatal(err)
			}
			if len(markers) != 0 {
				t.Errorf("contains markers: %+v", markers)
			}
		})
	}
}
