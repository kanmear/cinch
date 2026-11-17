package cinch

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"cinch/internal/mdscan"
	"cinch/internal/output"
)

// moveReport lists, as slash-separated repo-relative paths, what one move did.
// rewrites names each file whose links changed, by the path it has after the
// move.
type moveReport struct {
	source   string
	target   string
	rewrites []fileRewrite
}

type fileRewrite struct {
	file  string
	links int
}

// pendingWrite is one file's new content, computed before anything is written
// so a failure while planning leaves the tree untouched.
type pendingWrite struct {
	path  string
	data  []byte
	mode  fs.FileMode
	links int
}

// linkEdit replaces line[start:end], the path part of one link destination.
type linkEdit struct {
	start int
	end   int
	text  string
}

const moveReminder = "links from outside the docs root (README.md, AGENTS.md, code comments) are not rewritten; grep for"

func CmdMv(repoRoot, source, target string) int {
	docsRoot, err := ResolveDocsRoot(repoRoot)
	if err != nil {
		return output.Fail("mv", err)
	}
	report, err := moveFile(repoRoot, docsRoot, source, target)
	if err != nil {
		return output.Fail("mv", err)
	}
	printMoveReport(report)
	printMoveReminder([]string{report.source})
	return 0
}

func printMoveReport(report moveReport) {
	fmt.Printf("moved %s -> %s\n", report.source, report.target)
	for _, rewrite := range report.rewrites {
		fmt.Printf("rewrote %s (%d %s)\n", rewrite.file, rewrite.links, output.Plural(rewrite.links, "link"))
	}
}

func printMoveReminder(oldPaths []string) {
	quoted := make([]string, len(oldPaths))
	for i, old := range oldPaths {
		quoted[i] = "'" + old + "'"
	}
	fmt.Printf("note: %s %s\n", moveReminder, strings.Join(quoted, ", "))
}

// moveFile moves source to target, both as the user typed them (relative to
// repoRoot), and rewrites every link the move would break. Everything is
// validated and planned in memory first; the writes come last.
func moveFile(repoRoot, docsRoot, source, target string) (moveReport, error) {
	absRepo, absDocs, absSource, absTarget, err := validateMove(repoRoot, docsRoot, source, target)
	if err != nil {
		return moveReport{}, err
	}
	info, err := os.Stat(absSource)
	if err != nil {
		return moveReport{}, err
	}
	data, err := os.ReadFile(absSource)
	if err != nil {
		return moveReport{}, err
	}

	oldDirectory, newDirectory := filepath.Dir(absSource), filepath.Dir(absTarget)
	movedData, movedLinks := rewriteLinks(data, oldDirectory, func(pathPart, resolved string) (string, bool) {
		if _, err := os.Stat(resolved); err != nil {
			return "", false
		}
		mapped := resolved
		if resolved == absSource {
			mapped = absTarget
		}
		if filepath.Join(newDirectory, filepath.FromSlash(pathPart)) == mapped {
			return "", false
		}
		return relativeLink(newDirectory, mapped), true
	})
	writes := []pendingWrite{{path: absTarget, data: movedData, mode: info.Mode().Perm(), links: movedLinks}}

	var others []pendingWrite
	err = mdscan.WalkMarkdownFiles(absDocs, func(path string) error {
		if path == absSource {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rewritten, links := rewriteLinks(content, filepath.Dir(path), func(pathPart, resolved string) (string, bool) {
			if resolved != absSource {
				return "", false
			}
			return relativeLink(filepath.Dir(path), absTarget), true
		})
		if links == 0 {
			return nil
		}
		fileInfo, err := os.Stat(path)
		if err != nil {
			return err
		}
		others = append(others, pendingWrite{path: path, data: rewritten, mode: fileInfo.Mode().Perm(), links: links})
		return nil
	})
	if err != nil {
		return moveReport{}, err
	}
	writes = append(writes, others...)

	if err := os.MkdirAll(newDirectory, 0o755); err != nil {
		return moveReport{}, err
	}
	if err := os.WriteFile(absTarget, writes[0].data, writes[0].mode); err != nil {
		return moveReport{}, err
	}
	if err := os.Remove(absSource); err != nil {
		return moveReport{}, err
	}
	for _, write := range others {
		if err := os.WriteFile(write.path, write.data, write.mode); err != nil {
			return moveReport{}, err
		}
	}

	report := moveReport{source: repoRelative(absRepo, absSource), target: repoRelative(absRepo, absTarget)}
	for _, write := range writes {
		if write.links > 0 {
			report.rewrites = append(report.rewrites, fileRewrite{file: repoRelative(absRepo, write.path), links: write.links})
		}
	}
	return report, nil
}

// validateMove returns the absolute paths it checked, so callers don't resolve
// them twice. It reads the tree but never writes it.
func validateMove(repoRoot, docsRoot, source, target string) (absRepo, absDocs, absSource, absTarget string, err error) {
	if absRepo, err = filepath.Abs(repoRoot); err != nil {
		return
	}
	if absDocs, err = filepath.Abs(docsRoot); err != nil {
		return
	}
	absSource = resolveRepoPath(absRepo, source)
	absTarget = resolveRepoPath(absRepo, target)
	docsName := repoRelative(absRepo, absDocs)

	info, statErr := os.Stat(absSource)
	switch {
	case statErr != nil && os.IsNotExist(statErr):
		err = fmt.Errorf("source does not exist: %s", source)
	case statErr != nil:
		err = statErr
	case !info.Mode().IsRegular():
		err = fmt.Errorf("source is not a regular file: %s", source)
	case !strings.HasSuffix(absSource, ".md"):
		err = fmt.Errorf("source is not a .md file: %s", source)
	case !insideDirectory(absDocs, absSource):
		err = fmt.Errorf("source is outside the docs root (%s): %s", docsName, source)
	case !strings.HasSuffix(absTarget, ".md"):
		err = fmt.Errorf("target is not a .md file: %s", target)
	case !insideDirectory(absDocs, absTarget):
		err = fmt.Errorf("target is outside the docs root (%s): %s", docsName, target)
	default:
		if _, lstatErr := os.Lstat(absTarget); lstatErr == nil {
			err = fmt.Errorf("target already exists: %s", target)
		} else if !os.IsNotExist(lstatErr) {
			err = fmt.Errorf("target's parent is not a directory: %s", target)
		} else {
			err = checkTargetDirectory(filepath.Dir(absTarget), target)
		}
	}
	return
}

// checkTargetDirectory finds the nearest existing ancestor of directory and
// requires it to be a directory, so the later MkdirAll cannot fail halfway.
func checkTargetDirectory(directory, target string) error {
	for {
		info, err := os.Stat(directory)
		if err == nil {
			if !info.IsDir() {
				return fmt.Errorf("target's parent is not a directory: %s", target)
			}
			return nil
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return nil
		}
		directory = parent
	}
}

func resolveRepoPath(absRepo, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(absRepo, path)
}

func insideDirectory(directory, path string) bool {
	rel, err := filepath.Rel(directory, path)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func repoRelative(absRepo, path string) string {
	return filepath.ToSlash(mdscan.RelTo(absRepo, path))
}

func relativeLink(fromDirectory, to string) string {
	return filepath.ToSlash(mdscan.RelTo(fromDirectory, to))
}

// rewriteLinks finds links the way the links check does and asks rewrite for a
// new path part for each one. rewrite gets the link's path part (fragment cut
// off) and where it resolves from directory. Only that path part is replaced,
// so a #fragment, a title and <...> brackets survive. It returns the new
// content and how many links changed.
func rewriteLinks(data []byte, directory string, rewrite func(pathPart, resolved string) (string, bool)) ([]byte, int) {
	lines := strings.Split(string(data), "\n")
	edits := make(map[int][]linkEdit)
	count := 0
	// A scan error can only come from the reader, and a bytes.Reader has none.
	_ = mdscan.ForEachFencedLineBytes(data, func(lineNumber int, line string) {
		for _, m := range mdLinkRe.FindAllStringSubmatchIndex(stripInlineCode(line), -1) {
			raw := line[m[2]:m[3]]
			target := linkTargetFrom(raw)
			if linkTargetIsExempt(target) {
				continue
			}
			pathPart, _, _ := strings.Cut(target, "#")
			if pathPart == "" {
				continue
			}
			text, changed := rewrite(pathPart, filepath.Join(directory, filepath.FromSlash(pathPart)))
			if !changed {
				continue
			}
			start := m[2] + len(raw) - len(strings.TrimLeft(raw, " \t\r\n"))
			if line[start] == '<' {
				start++
			}
			edits[lineNumber-1] = append(edits[lineNumber-1], linkEdit{start: start, end: start + len(pathPart), text: text})
			count++
		}
	})
	if count == 0 {
		return data, 0
	}
	for index, lineEdits := range edits {
		line := lines[index]
		for i := len(lineEdits) - 1; i >= 0; i-- {
			e := lineEdits[i]
			line = line[:e.start] + e.text + line[e.end:]
		}
		lines[index] = line
	}
	return []byte(strings.Join(lines, "\n")), count
}
