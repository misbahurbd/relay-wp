package service

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

var (
	casesDocTestLine = regexp.MustCompile(`\bTest:\s+(.+)$`)
	casesDocMarked   = regexp.MustCompile(`^\d+\. (✅|🔧)`)
	casesDocTestName = regexp.MustCompile(`^Test[A-Za-z0-9_]*$`)
	casesDocTestFunc = regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]*)\(t \*testing\.T\)`)
)

// TestBillingCasesDoc keeps docs/BILLING-CASES.md honest. Every case marked
// built or live-tested ends with "Test: Name, Name", and each name must be a
// test function somewhere under apps/api. "none" and "pending" are placeholders.
func TestBillingCasesDoc(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot find this test file")
	}
	apiRoot := filepath.Join(filepath.Dir(here), "..", "..", "..", "..")
	doc, err := os.ReadFile(filepath.Join(apiRoot, "..", "..", "docs", "BILLING-CASES.md")) //nolint:gosec // a fixed path in this repo
	if err != nil {
		t.Fatalf("read the cases doc: %v", err)
	}

	named := map[string][]string{}
	for line := range strings.SplitSeq(string(doc), "\n") {
		m := casesDocTestLine.FindStringSubmatch(line)
		if m == nil {
			if casesDocMarked.MatchString(line) {
				t.Errorf("a built or live-tested case has no \"Test:\" ending: %.70s", line)
			}
			continue
		}
		for _, name := range strings.Split(strings.TrimSuffix(strings.TrimSpace(m[1]), "."), ",") {
			name = strings.Trim(strings.TrimSpace(name), "`")
			switch name {
			case "none", "pending":
				continue
			}
			if !casesDocTestName.MatchString(name) {
				t.Errorf("%q after \"Test:\" is not a test name, in: %.70s", name, line)
				continue
			}
			named[name] = append(named[name], strings.TrimSuffix(line[:strings.IndexByte(line, ' ')], "."))
		}
	}
	if len(named) == 0 {
		t.Fatal("the cases doc names no tests; the \"Test:\" format changed")
	}

	defined := map[string]bool{}
	err = filepath.WalkDir(apiRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path) //nolint:gosec // a test file found by walking this repo
		if err != nil {
			return err
		}
		for _, m := range casesDocTestFunc.FindAllSubmatch(src, -1) {
			defined[string(m[1])] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk apps/api: %v", err)
	}

	var missing []string
	for name, cases := range named {
		if !defined[name] {
			missing = append(missing, name+" (case "+strings.Join(cases, ", ")+")")
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("docs/BILLING-CASES.md names tests that don't exist:\n\t%s", strings.Join(missing, "\n\t"))
	}
}
