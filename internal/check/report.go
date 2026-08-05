package check

import (
	"fmt"
	"sort"
)

type finding struct {
	code  string
	level string // "error" | "warn"
	msg   string
}

type report struct{ findings []finding }

func (r *report) errf(code, format string, a ...any) {
	r.findings = append(r.findings, finding{code, "error", fmt.Sprintf(format, a...)})
}

func (r *report) warnf(code, format string, a ...any) {
	r.findings = append(r.findings, finding{code, "warn", fmt.Sprintf(format, a...)})
}

// emit prints the report: errors sorted by code, exit status from errors.
func emit(r *report) error {
	sort.SliceStable(r.findings, func(i, j int) bool { return r.findings[i].code < r.findings[j].code })

	errs := 0
	for _, f := range r.findings {
		if f.level == "error" {
			errs++
		}
		fmt.Printf("%-5s %-5s %s\n", f.code, f.level, f.msg)
	}
	if len(r.findings) == 0 {
		fmt.Println("harness ok")
		return nil
	}
	if errs > 0 {
		return fmt.Errorf("%d error(s), %d warning(s)", errs, len(r.findings)-errs)
	}
	fmt.Printf("\n%d warning(s), no errors\n", len(r.findings))
	return nil
}
