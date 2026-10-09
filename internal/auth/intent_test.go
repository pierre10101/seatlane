package auth

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The bridge-en I1-I3 discipline, for app code bridge-en does not read:
// each endpoint directory has an intent.md with exactly one "## Failure
// cases" section in the I2 format; its F-IDs are exactly the ones Failures
// declares for it; and every F-ID has a func TestF<n>_ in this package that
// names it.
func TestIntentsMatchCodeAndChecks(t *testing.T) {
	entry := regexp.MustCompile(`^- (F[1-9][0-9]*): \S`)
	srcs, _ := filepath.Glob("*_test.go")
	var tests strings.Builder
	for _, f := range srcs {
		b, _ := os.ReadFile(f)
		tests.Write(b)
	}
	for dir, declared := range Failures {
		f, err := os.Open(filepath.Join(dir, "intent.md"))
		if err != nil {
			t.Fatalf("%s: no intent.md (I1): %v", dir, err)
		}
		var listed []string
		sections, in, none := 0, false, false
		sc := bufio.NewScanner(f)
		for n := 1; sc.Scan(); n++ {
			line := sc.Text()
			switch {
			case strings.HasPrefix(line, "#"):
				in = line == "## Failure cases"
				if in {
					sections++
				}
			case !in && entry.MatchString(line):
				t.Errorf("%s/intent.md:%d: failure case outside \"## Failure cases\" (I2)", dir, n)
			case !in || line == "":
			case line == "None.":
				none = true
			case strings.HasPrefix(line, "  "):
			case entry.MatchString(line):
				listed = append(listed, entry.FindStringSubmatch(line)[1])
			default:
				t.Errorf("%s/intent.md:%d: %q is not \"- F<n>: <text>\" (I2)", dir, n, line)
			}
		}
		f.Close()
		if sections != 1 || (len(listed) == 0) != none {
			t.Fatalf("%s/intent.md: %d failure sections, none=%v, listed %v (I2)", dir, sections, none, listed)
		}
		var ids []string
		for _, d := range declared {
			ids = append(ids, d.ID)
		}
		if !sort.SliceIsSorted(listed, func(i, j int) bool { return num(listed[i]) < num(listed[j]) }) {
			t.Errorf("%s/intent.md: F-IDs not in increasing order: %v (I2)", dir, listed)
		}
		if strings.Join(listed, ",") != strings.Join(ids, ",") {
			t.Errorf("%s: intent.md lists %v, code declares %v (I3)", dir, listed, ids)
		}
		for _, id := range ids {
			if !regexp.MustCompile(`func Test` + id + `_\w+\(t \*testing\.T\) \{(?s:.*?)\b` + id + `\b`).MatchString(tests.String()) {
				t.Errorf("%s: no check func Test%s_... that references %s (I3)", dir, id, id)
			}
		}
	}
}

func num(id string) int {
	n := 0
	for _, c := range id[1:] {
		n = n*10 + int(c-'0')
	}
	return n
}
