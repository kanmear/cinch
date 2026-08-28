package cinch

import (
	"bufio"
	"bytes"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// docFrontmatter is the owns:/rule_prefix: convention documented for
// consumers in internal/cinch/docs/templates/docs-maintain-domain.md.
type docFrontmatter struct {
	Owns       []string `yaml:"owns"`
	RulePrefix string   `yaml:"rule_prefix"`
}

// parseFrontmatter extracts a leading `---`-delimited YAML block from a
// doc's content. The common case today is no frontmatter at all, and an
// absent or unterminated block returns a zero-value docFrontmatter with no
// error — only malformed YAML inside a well-delimited block is an error,
// since a typo'd block silently producing an empty Owns is a worse failure
// mode than a loud one.
func parseFrontmatter(content []byte) (docFrontmatter, error) {
	var fm docFrontmatter

	scanner := bufio.NewScanner(bytes.NewReader(content))
	if !scanner.Scan() || strings.TrimRight(scanner.Text(), "\r") != "---" {
		return fm, nil
	}

	var block bytes.Buffer
	closed := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimRight(line, "\r") == "---" {
			closed = true
			break
		}
		block.WriteString(line)
		block.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return fm, err
	}
	if !closed {
		return docFrontmatter{}, nil
	}

	if err := yaml.Unmarshal(block.Bytes(), &fm); err != nil {
		return docFrontmatter{}, err
	}
	return fm, nil
}

func parseFrontmatterFile(path string) (docFrontmatter, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return docFrontmatter{}, err
	}
	return parseFrontmatter(data)
}
