package cinch

import (
	"bytes"
	"io"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
)

var fenceRe = regexp.MustCompile("^\\s*```")

func forEachLineReader(r io.Reader, fn func(lineNumber int, line string)) error {
	const chunkSize = 64 << 10
	buffer := make([]byte, 0, chunkSize)
	temp := make([]byte, chunkSize)
	lineNumber := 0
	start := 0
	empties := 0

	flush := func(end int) {
		lineNumber++
		line := buffer[start:end]
		if n := len(line); n > 0 && line[n-1] == '\r' {
			line = line[:n-1]
		}
		fn(lineNumber, string(line))
		start = end + 1
	}

	for {
		n, err := r.Read(temp)
		if n > 0 {
			empties = 0
			base := len(buffer)
			buffer = append(buffer, temp[:n]...)
			for i := base; i < len(buffer); i++ {
				if buffer[i] == '\n' {
					flush(i)
				}
			}
			if start > 0 {
				copy(buffer, buffer[start:])
				buffer = buffer[:len(buffer)-start]
				start = 0
			}
		} else if err == nil {
			empties++
			if empties > 100 {
				return io.ErrNoProgress
			}
		}
		if err != nil {
			if err == io.EOF {
				if start < len(buffer) {
					flush(len(buffer))
				}
				return nil
			}
			return err
		}
	}
}

func forEachFencedLineReader(r io.Reader, fn func(lineNumber int, line string)) error {
	inFence := false
	return forEachLineReader(r, func(lineNumber int, line string) {
		if fenceRe.MatchString(line) {
			inFence = !inFence
			return
		}
		if inFence {
			return
		}
		fn(lineNumber, line)
	})
}

func forEachFencedLine(data []byte, fn func(lineNumber int, line string)) error {
	return forEachFencedLineReader(bytes.NewReader(data), fn)
}

func walkMarkdownFiles(root string, fn func(path string) error) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		return fn(path)
	})
}

func collectMarkdown[T any](root string, fn func(path string) ([]T, error)) ([]T, error) {
	var out []T
	err := walkMarkdownFiles(root, func(path string) error {
		more, err := fn(path)
		out = append(out, more...)
		return err
	})
	return out, err
}

// relTo returns path relative to base (best-effort: git's own output is
// always repo-relative, but filesystem walks may produce absolute paths).
func relTo(base, path string) string {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return path
	}
	return rel
}
