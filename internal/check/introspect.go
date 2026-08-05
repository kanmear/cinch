package check

import (
	"regexp"
	"strings"
)

// C6/C7 doc-matching helpers. These parse the consumer's *docs* (markdown
// under .agent/), never project source — source introspection is the
// manifest-declared producer (standing rule 5, D028).

var (
	routeHeadingRe = regexp.MustCompile("^##\\s+(GET|POST|PUT|PATCH|DELETE|OPTIONS|HEAD)\\s+(\\S+)")
	typeStructRe   = regexp.MustCompile("^type\\s+(\\w+)\\s+struct")
	structFieldRe  = regexp.MustCompile("^\\s*([A-Za-z_]\\w*)\\s+\\S+\\s*(`[^`]*`)?")
	jsonTagRe      = regexp.MustCompile("`json:\"([^\"]*)\"`")
)

type docStruct struct {
	name   string
	fields []string
}

// docStructs extracts every type a models doc documents: a ```go fenced block
// containing a `type <Name> struct` definition, whose listed fields are the
// doc's claim about that type (docs/INTROSPECTION.md). Brace depth tracks the
// block end, so a doc may document several structs — each in its own fence or
// one after another.
func docStructs(path string) []docStruct {
	return docStructsFromText(readFile(path))
}

func docStructsFromText(text string) []docStruct {
	var out []docStruct
	inFence := false
	var cur *docStruct
	depth := 0

	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			cur = nil
			continue
		}
		if !inFence {
			continue
		}
		if cur == nil {
			if m := typeStructRe.FindStringSubmatch(line); m != nil {
				out = append(out, docStruct{name: m[1]})
				cur = &out[len(out)-1]
				depth = strings.Count(line, "{") - strings.Count(line, "}")
				if depth <= 0 {
					cur = nil
				}
			}
			continue
		}
		if f, ok := fieldName(line); ok {
			cur.fields = append(cur.fields, f)
		}
		depth += strings.Count(line, "{") - strings.Count(line, "}")
		if depth <= 0 {
			cur = nil
		}
	}
	return out
}

// fieldName reads a Go struct field line: the json tag value when present and
// not "-", otherwise the declared Go name (a `json:"-"` field still exists and
// must be checkable — docs document PasswordHash even though it never
// serializes).
func fieldName(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "type ") || strings.HasPrefix(trimmed, "//") {
		return "", false
	}
	m := structFieldRe.FindStringSubmatch(line)
	if m == nil {
		return "", false
	}
	if m[2] != "" {
		if t := jsonTagRe.FindStringSubmatch(m[2]); t != nil && t[1] != "-" {
			return t[1], true
		}
	}
	return m[1], true
}
