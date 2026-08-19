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

func forEachLineReader(r io.Reader, fn func(lineNo int, line string)) error {
	const chunkSize = 64 << 10
	buf := make([]byte, 0, chunkSize)
	tmp := make([]byte, chunkSize)
	lineNo := 0
	start := 0
	empties := 0

	flush := func(end int) {
		lineNo++
		line := buf[start:end]
		if n := len(line); n > 0 && line[n-1] == '\r' {
			line = line[:n-1]
		}
		fn(lineNo, string(line))
		start = end + 1
	}

	for {
		n, err := r.Read(tmp)
		if n > 0 {
			empties = 0
			base := len(buf)
			buf = append(buf, tmp[:n]...)
			for i := base; i < len(buf); i++ {
				if buf[i] == '\n' {
					flush(i)
				}
			}
			if start > 0 {
				copy(buf, buf[start:])
				buf = buf[:len(buf)-start]
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
				if start < len(buf) {
					flush(len(buf))
				}
				return nil
			}
			return err
		}
	}
}

func forEachFencedLineReader(r io.Reader, fn func(lineNo int, line string)) error {
	inFence := false
	return forEachLineReader(r, func(lineNo int, line string) {
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

func forEachFencedLine(data []byte, fn func(lineNo int, line string)) error {
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
