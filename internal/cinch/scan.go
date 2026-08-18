package cinch

import (
	"bufio"
	"bytes"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
)

var fenceRe = regexp.MustCompile("^\\s*```")

// forEachLine calls fn for each line of data (1-indexed line numbers).
// It returns the scanner error, if any.
func forEachLine(data []byte, fn func(lineNo int, line string)) error {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		fn(lineNo, scanner.Text())
	}
	return scanner.Err()
}

// forEachFencedLine calls fn for each line of data that is not inside a
// fenced code block (``` toggles fence state).
func forEachFencedLine(data []byte, fn func(lineNo int, line string)) error {
	inFence := false
	return forEachLine(data, func(lineNo int, line string) {
		if fenceRe.MatchString(line) {
			inFence = !inFence
			return
		}
		if inFence {
			return
		}
		fn(lineNo, line)
	})
}

// walkMarkdownFiles calls fn for every *.md file under root.
func walkMarkdownFiles(root string, fn func(path string) error) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		return fn(path)
	})
}
