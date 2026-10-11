package cinch

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"cinch/internal/concurrency"
	"cinch/internal/gitutil"
	"cinch/internal/mdscan"
	"cinch/internal/output"
)

var (
	ruleItemRe = regexp.MustCompile(`^\s*\d+\.\s+\*\*([A-Z0-9]+-[0-9]+)\*\*`)
	anyItemRe  = regexp.MustCompile(`^\s*\d+\.\s`)
	ignoreRe   = regexp.MustCompile(`<!--\s*cinch:ignore\s*(?::\s*(.*?))?\s*-->`)
	markerRe   = regexp.MustCompile(`(?://|#|--|<!--|/\*|%|;)\s*cinch:rule\s+([A-Z0-9]+-[0-9]+)\b`)
)

type ruleItem struct {
	id   string
	file string
	line int

	text string

	hasIgnore    bool
	ignoreReason string
	ignoreLine   int
}

type markerLoc struct {
	file string
	line int
}

type rulesReport struct {
	findings []finding
	rules    int
	docs     int
	ignores  int
	// skip, when non-empty, means the check couldn't meaningfully run at all
	// (every configured rules.roots entry is absent) — reported as a single
	// "skip: ..." rather than one misleading "no marker found" per rule.
	skip string
}

// rulesRootsKey configures additional sibling filesystem roots (relative
// paths, ".." allowed) to also scan for // cinch:rule markers — for a docs
// repo that no longer contains the code it governs. Unlike paths.docs/
// paths.hooks (manifest.go's repoLocalPathKeys), these are read-only scan
// targets, not write destinations, so leaving the repository is the point.
const rulesRootsKey = "rules.roots"

func markerScanOptionsFor(m *manifest) markerScanOptions {
	return markerScanOptions{extraRoots: m.list(rulesRootsKey), exclude: m.list(pathsExcludeKey)}
}

func ruleItemIDs(items []ruleItem) map[string]bool {
	ids := make(map[string]bool, len(items))
	for _, item := range items {
		ids[item.id] = true
	}
	return ids
}

// levenshtein returns the edit distance between a and b (insertions,
// deletions, substitutions each cost 1).
func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	curr := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		curr[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			min := del
			if ins < min {
				min = ins
			}
			if sub < min {
				min = sub
			}
			curr[j] = min
		}
		prev, curr = curr, prev
	}
	return prev[len(rb)]
}

// nearMissRuleIDs returns up to 2 candidate IDs from ruleIDs that are
// plausible typos of id (edit distance <= 2), sorted by (distance, id) for
// determinism across runs despite ruleIDs being an unordered map. Callers
// only invoke this on an id that is not itself present in ruleIDs.
func nearMissRuleIDs(id string, ruleIDs map[string]bool) []string {
	type candidate struct {
		id       string
		distance int
	}
	var candidates []candidate
	for other := range ruleIDs {
		if d := levenshtein(id, other); d <= 2 {
			candidates = append(candidates, candidate{id: other, distance: d})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].distance != candidates[j].distance {
			return candidates[i].distance < candidates[j].distance
		}
		return candidates[i].id < candidates[j].id
	})
	if len(candidates) == 0 {
		return nil
	}
	if len(candidates) > 2 {
		candidates = candidates[:2]
	}
	out := make([]string, len(candidates))
	for i, c := range candidates {
		out[i] = c.id
	}
	return out
}

// duplicateLocsMessage describes every location that declared the same rule
// ID, for a "declared in N places" finding. locs must have at least 2
// elements.
func duplicateLocsMessage(locs []ruleItem) string {
	pairs := make([]string, len(locs))
	for i, loc := range locs {
		pairs[i] = fmt.Sprintf("%s:%d", loc.file, loc.line)
	}
	if len(pairs) == 2 {
		return "declared in two places (" + pairs[0] + " and " + pairs[1] + ") — rule IDs must be unique"
	}
	return fmt.Sprintf("declared in %d places (%s) — rule IDs must be unique", len(pairs), strings.Join(pairs, ", "))
}

func parseRuleItems(file string, content []byte) []ruleItem {
	var items []ruleItem
	var cur *ruleItem
	var textLines []string

	flush := func() {
		if cur == nil {
			return
		}
		cur.text = strings.Join(strings.Fields(strings.Join(textLines, " ")), " ")
		items = append(items, *cur)
		cur = nil
		textLines = nil
	}

	_ = mdscan.ForEachFencedLineBytes(content, func(lineNumber int, line string) {
		if m := ruleItemRe.FindStringSubmatch(line); m != nil {
			flush()
			cur = &ruleItem{id: m[1], file: file, line: lineNumber}
			textLines = []string{line}
			return
		}
		if anyItemRe.MatchString(line) {
			flush()
			return
		}
		if cur == nil {
			return
		}
		textLines = append(textLines, line)
		if !cur.hasIgnore {
			if m := ignoreRe.FindStringSubmatch(line); m != nil {
				cur.hasIgnore = true
				cur.ignoreReason = strings.TrimSpace(m[1])
				cur.ignoreLine = lineNumber
			}
		}
	})
	flush()

	return items
}

func parseRuleDoc(path string) ([]ruleItem, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseRuleItems(path, data), nil
}

func scanRuleDocs(roots checkRoots, docsRoot string) ([]ruleItem, error) {
	return collectDocs(roots, docsRoot, parseRuleDoc)
}

func underPath(path, directory string) bool {
	if directory == "" {
		return false
	}
	absPath, err1 := filepath.Abs(path)
	absDirectory, err2 := filepath.Abs(directory)
	if err1 != nil || err2 != nil {
		return false
	}
	rel, err := filepath.Rel(absDirectory, absPath)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}

// ruleScanRoot is one filesystem location scanned for // cinch:rule markers.
// display is the prefix applied to a file's reported path — empty for the
// primary repoRoot (reported paths stay exactly as before), or the
// configured rules.roots entry for a sibling (e.g. "../backend"), so a
// finding stays navigable from wherever cinch was invoked. files, when
// non-nil, is the precomputed repo-relative list to scan (the index, in
// staged mode) instead of asking git or walking path. exclude holds the
// paths.exclude entries; only the primary root carries them, since they are
// written relative to this repository.
type ruleScanRoot struct {
	path    string
	display string
	files   []string
	exclude []string
}

// markerScanOptions is what the manifest contributes to a marker scan:
// extraRoots from rules.roots, exclude from paths.exclude.
type markerScanOptions struct {
	extraRoots []string
	exclude    []string
}

// scannedFile pairs where to actually open a candidate file (fsPath) with
// where to report it (display) — they differ only for a sibling root.
type scannedFile struct {
	fsPath  string
	display string
}

// scanFileError is a per-file scan failure. It keeps the file's display path
// so checkRules can report the finding at the file that failed instead of at
// the repository root.
type scanFileError struct {
	display string
	err     error
}

func (e *scanFileError) Error() string { return e.err.Error() }
func (e *scanFileError) Unwrap() error { return e.err }

// resolveRuleScanRoots turns rules.roots entries into scan roots, splitting
// out any that don't exist on disk (a docs-only clone, or a CI job that only
// checked out one repo) so checkRules can report a clean skip instead of one
// false "no marker found" finding per rule. Siblings resolve against the real
// repoRoot even in staged mode: they are other repositories, scanned in their
// current state.
func resolveRuleScanRoots(roots checkRoots, options markerScanOptions) (present []ruleScanRoot, missing []string) {
	present = append(present, ruleScanRoot{path: roots.fsRoot, files: roots.indexFiles, exclude: options.exclude})
	for _, r := range options.extraRoots {
		resolved := filepath.Join(roots.repoRoot, r)
		if info, err := os.Stat(resolved); err == nil && info.IsDir() {
			present = append(present, ruleScanRoot{path: resolved, display: r})
		} else {
			missing = append(missing, r)
		}
	}
	return present, missing
}

// scanCandidates returns root's repo-relative file list: the precomputed one
// when set, otherwise git's view of root.path.
func scanCandidates(root ruleScanRoot) ([]string, error) {
	if root.files != nil {
		return root.files, nil
	}
	// A sibling is another repository, so it must not see the git
	// environment a hook inherited for this one.
	if root.display != "" {
		return gitutil.ForeignScannableFiles(root.path)
	}
	return gitutil.ScannableFiles(root.path)
}

func markerScanFiles(root ruleScanRoot, docsRoot string) ([]scannedFile, error) {
	toScanned := func(rel string) scannedFile {
		display := rel
		if root.display != "" {
			display = filepath.ToSlash(filepath.Join(root.display, rel))
		}
		return scannedFile{fsPath: filepath.Join(root.path, rel), display: display}
	}

	if files, err := scanCandidates(root); err == nil {
		var out []scannedFile
		for _, f := range files {
			sf := toScanned(f)
			if underPath(sf.fsPath, docsRoot) || excludedPath(f, root.exclude) {
				continue
			}
			// Skip a candidate with no file content to scan:
			//  - A directory. git ls-files lists a submodule as one gitlink
			//    entry, a path that is a directory on disk, not a blob. Opening
			//    it would succeed (Linux permits open() on a directory) but
			//    fail on the first read with EISDIR, surfacing as a bogus scan
			//    error. rules.roots is the supported way to also scan a
			//    submodule's own content. A symlink to a directory fails the
			//    same way, so os.Stat follows the link.
			//  - A path missing from disk. ls-files --cached still lists a
			//    tracked file that was deleted but not staged, and an index
			//    snapshot holds no entry for a gitlink (checkout-index doesn't
			//    write one). Either way the working tree has nothing to read.
			// A dangling symlink is not missing: Lstat sees the link itself,
			// so it stays a candidate and fails visibly when opened.
			info, statErr := os.Lstat(sf.fsPath)
			if errors.Is(statErr, fs.ErrNotExist) {
				continue
			}
			if statErr == nil && info.Mode()&os.ModeSymlink != 0 {
				if target, err := os.Stat(sf.fsPath); err == nil {
					info = target
				}
			}
			if statErr == nil && info.IsDir() {
				continue
			}
			out = append(out, sf)
		}
		return out, nil
	}

	var out []scannedFile
	err := filepath.WalkDir(root.path, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || underPath(path, docsRoot) {
				return filepath.SkipDir
			}
			return nil
		}
		if underPath(path, docsRoot) {
			return nil
		}
		rel, err := filepath.Rel(root.path, path)
		if err != nil {
			rel = path
		}
		if excludedPath(rel, root.exclude) {
			return nil
		}
		out = append(out, toScanned(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// excludedPath reports whether the root-relative file rel matches a
// paths.exclude entry: an entry ending in "/" is a directory prefix, any
// other is a path.Match pattern (so a plain path matches exactly, and "*"
// never crosses "/").
func excludedPath(rel string, exclude []string) bool {
	rel = filepath.ToSlash(rel)
	for _, entry := range exclude {
		if strings.HasSuffix(entry, "/") {
			if strings.HasPrefix(rel, entry) {
				return true
			}
			continue
		}
		if ok, _ := path.Match(entry, rel); ok {
			return true
		}
	}
	return false
}

func scanMarkers(r io.Reader, path string, fenced bool) (map[string][]markerLoc, error) {
	markers := map[string][]markerLoc{}
	fn := func(lineNumber int, line string) {
		if m := markerRe.FindStringSubmatch(line); m != nil {
			markers[m[1]] = append(markers[m[1]], markerLoc{file: path, line: lineNumber})
		}
	}
	var err error
	if fenced {
		err = mdscan.ForEachFencedLine(r, fn)
	} else {
		err = mdscan.ForEachLine(r, fn)
	}
	return markers, err
}

// binarySniffLen is how much of a file git reads to decide it is binary.
const binarySniffLen = 8 << 10

// markerNeedle is the literal every marker contains: markerRe allows nothing
// between "cinch:" and "rule". A file without it can't hold a marker.
var markerNeedle = []byte("cinch:rule")

// scanFileMarkers reads a file once and line-scans it only when it can hold
// a marker. A binary file (a NUL in its first binarySniffLen bytes, git's own
// test) is skipped before the rest of it is read: markers are comments, which
// don't survive compilation, and one 12 MB server binary scanned line by line
// took a consumer's rules check from 50 ms to 850 ms. Most text files hold no
// marker, so the needle check spares them the line scan, which was most of
// the hook's cost on large repos.
func scanFileMarkers(path string) (map[string][]markerLoc, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(make([]byte, 0, info.Size()+bytes.MinRead))
	if _, err := io.CopyN(buf, f, binarySniffLen); err != nil && err != io.EOF {
		return nil, err
	}
	if bytes.IndexByte(buf.Bytes(), 0) >= 0 {
		return map[string][]markerLoc{}, nil
	}
	if _, err := buf.ReadFrom(f); err != nil {
		return nil, err
	}
	if !bytes.Contains(buf.Bytes(), markerNeedle) {
		return map[string][]markerLoc{}, nil
	}
	return scanMarkers(bytes.NewReader(buf.Bytes()), path, strings.HasSuffix(path, ".md"))
}

// scanFileMarkersFunc is the per-file scan the worker pool calls; tests wrap
// it to count scans.
var scanFileMarkersFunc = scanFileMarkers

// markerScanCache keeps each root's marker scan for the length of one run,
// so the pre-commit hook's rules check and impact advisory share one pass
// over the tree instead of reading every file twice. It's keyed per scan
// root because the two scan different root sets: impact leaves out
// rules.roots siblings. A nil cache scans every time.
type markerScanCache struct {
	mu    sync.Mutex
	roots map[string]map[string][]markerLoc
}

func newMarkerScanCache() *markerScanCache {
	return &markerScanCache{roots: map[string]map[string][]markerLoc{}}
}

// scan returns root's markers. Callers must not modify the result: it may be
// the cached map another check also reads.
func (c *markerScanCache) scan(root ruleScanRoot, docsRoot string) (map[string][]markerLoc, error) {
	if c == nil {
		return scanRootMarkers(root, docsRoot)
	}
	key := strings.Join(append([]string{root.path, root.display, docsRoot}, root.exclude...), "\x00")
	c.mu.Lock()
	found, ok := c.roots[key]
	c.mu.Unlock()
	if ok {
		return found, nil
	}
	found, err := scanRootMarkers(root, docsRoot)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.roots[key] = found
	c.mu.Unlock()
	return found, nil
}

// scanRuleMarkers reads every in-scope file (roots.fsRoot plus any present
// rules.roots sibling) looking for // cinch:rule markers, reusing a root's
// scan from roots.scans when an earlier check already did it. Each markers[id]
// is a fresh slice, sorted by file, then line: the same tree must give the
// same output. missingRoots names any configured extraRoots entry that
// doesn't exist on disk, so the caller can decide whether to skip rather than
// report.
func scanRuleMarkers(roots checkRoots, docsRoot string, options markerScanOptions) (markers map[string][]markerLoc, missingRoots []string, err error) {
	scanRoots, missing := resolveRuleScanRoots(roots, options)

	out := map[string][]markerLoc{}
	for _, root := range scanRoots {
		found, err := roots.scans.scan(root, docsRoot)
		if err != nil {
			return nil, nil, err
		}
		for id, locs := range found {
			out[id] = append(out[id], locs...)
		}
	}
	for _, locs := range out {
		sort.Slice(locs, func(i, j int) bool {
			if locs[i].file != locs[j].file {
				return locs[i].file < locs[j].file
			}
			return locs[i].line < locs[j].line
		})
	}
	return out, missing, nil
}

// scanRootMarkers scans one root's candidate files with a bounded worker
// pool, since each file is an independent open and scan. Workers finish in
// any order; scanRuleMarkers sorts the merged result.
func scanRootMarkers(root ruleScanRoot, docsRoot string) (map[string][]markerLoc, error) {
	files, err := markerScanFiles(root, docsRoot)
	if err != nil {
		return nil, err
	}
	out := map[string][]markerLoc{}
	if len(files) == 0 {
		return out, nil
	}

	type fileResult struct {
		markers map[string][]markerLoc
		err     error
	}

	jobs := make(chan scannedFile)
	results := make(chan fileResult)

	var wg sync.WaitGroup
	for i := 0; i < concurrency.BoundedWorkers(len(files)); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range jobs {
				m, ferr := scanFileMarkersFunc(f.fsPath)
				if ferr == nil {
					for id, locs := range m {
						relabeled := make([]markerLoc, len(locs))
						for i, loc := range locs {
							relabeled[i] = markerLoc{file: f.display, line: loc.line}
						}
						m[id] = relabeled
					}
				}
				if ferr != nil {
					ferr = &scanFileError{display: f.display, err: ferr}
				}
				results <- fileResult{markers: m, err: ferr}
			}
		}()
	}
	go func() {
		for _, f := range files {
			jobs <- f
		}
		close(jobs)
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	var firstErr error
	for r := range results {
		if r.err != nil {
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		for id, locs := range r.markers {
			out[id] = append(out[id], locs...)
		}
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return out, nil
}

// checkRules cross-references documented rule IDs under docsRoot against
// // cinch:rule markers under roots.fsRoot plus any extraRoots (rules.roots) —
// sibling repos for a docs corpus that no longer contains the code it
// governs. If every configured extraRoots entry is absent, every rule would
// otherwise report a misleading "no marker found", so the whole check is
// skipped instead: there's no code available to check against, not a real
// gap.
func checkRules(roots checkRoots, docsRoot string, options markerScanOptions) rulesReport {
	items, err := scanRuleDocs(roots, docsRoot)
	if err != nil {
		return rulesReport{findings: scanErrorFinding("rules", docsRoot, err)}
	}
	markers, missingRoots, err := scanRuleMarkers(roots, docsRoot, options)
	if err != nil {
		// A failure reading one file is reported at that file; anything else
		// (git failing, a walk error) has no file to name.
		scope := roots.repoRoot
		var fileErr *scanFileError
		if errors.As(err, &fileErr) {
			scope = fileErr.display
		}
		return rulesReport{findings: scanErrorFinding("rules", scope, err)}
	}
	if len(options.extraRoots) > 0 && len(missingRoots) == len(options.extraRoots) {
		return rulesReport{skip: "configured code roots not present (" + strings.Join(missingRoots, ", ") + ")"}
	}

	return checkRulesFrom(items, markers)
}

func rulesCheckResult(report rulesReport) checkResult {
	if report.skip != "" {
		return checkResult{noOp: report.skip}
	}
	if report.rules == 0 && len(report.findings) == 0 {
		return checkResult{noOp: "no rule IDs found — rules check enforces nothing"}
	}
	return checkResult{
		findings: report.findings,
		detail: fmt.Sprintf("(%d %s, %d %s, %d %s)",
			report.rules, output.Plural(report.rules, "rule"),
			report.docs, output.Plural(report.docs, "rule doc"),
			report.ignores, output.Plural(report.ignores, "ignore")),
	}
}

func checkRulesFrom(items []ruleItem, markers map[string][]markerLoc) rulesReport {
	var findings []finding

	for _, item := range items {
		_, marked := markers[item.id]

		if item.hasIgnore && item.ignoreReason == "" {
			findings = append(findings, finding{
				check: "rules", level: "error", file: item.file, line: item.ignoreLine,
				message: item.id + ": cinch:ignore has no reason",
			})
		}
		if item.hasIgnore && item.ignoreReason != "" && marked {
			findings = append(findings, finding{
				check: "rules", level: "error", file: item.file, line: item.line,
				message: item.id + ": declared cinch:ignore but also has a // cinch:rule marker — pick one",
			})
		}
		if !item.hasIgnore && !marked {
			findings = append(findings, finding{
				check: "rules", level: "error", file: item.file, line: item.line,
				message: item.id + ": no // cinch:rule marker (or cinch:ignore declaration)",
			})
		}
	}

	ruleIDs := ruleItemIDs(items)
	for id, locs := range markers {
		if ruleIDs[id] {
			continue
		}
		message := "// cinch:rule " + id + " does not resolve to any rule ID"
		if suggestions := nearMissRuleIDs(id, ruleIDs); len(suggestions) > 0 {
			message += " (did you mean " + strings.Join(suggestions, " or ") + "?)"
		}
		for _, loc := range locs {
			findings = append(findings, finding{
				check: "rules", level: "error", file: loc.file, line: loc.line,
				message: message,
			})
		}
	}

	docs := make(map[string]bool, len(items))
	byID := make(map[string][]ruleItem, len(items))
	ignores := 0
	for _, item := range items {
		docs[item.file] = true
		byID[item.id] = append(byID[item.id], item)
		if item.hasIgnore {
			ignores++
		}
	}

	dupIDs := make([]string, 0, len(byID))
	for id := range byID {
		dupIDs = append(dupIDs, id)
	}
	sort.Strings(dupIDs)
	for _, id := range dupIDs {
		locs := byID[id]
		if len(locs) <= 1 {
			continue
		}
		findings = append(findings, finding{
			check: "rules", level: "error", file: locs[0].file, line: locs[0].line,
			message: id + ": " + duplicateLocsMessage(locs),
		})
	}

	return rulesReport{
		findings: findings,
		rules:    len(items),
		docs:     len(docs),
		ignores:  ignores,
	}
}

func CmdIgnores(docsRoot string) int {
	items, err := scanRuleDocs(workingTreeRoots(docsRoot), docsRoot)
	if err != nil {
		return output.Fail("ignores", err)
	}
	var ignored []ruleItem
	for _, item := range items {
		if item.hasIgnore {
			ignored = append(ignored, item)
		}
	}
	if len(ignored) == 0 {
		fmt.Println("0 cinch:ignore declarations found")
		return 0
	}
	fmt.Printf("%d cinch:ignore declarations:\n\n", len(ignored))
	for _, item := range ignored {
		reason := item.ignoreReason
		if reason == "" {
			reason = "(no reason — malformed, see cinch check)"
		}
		fmt.Printf("%s %s: %s\n", item.id, item.file, reason)
	}
	return 0
}
